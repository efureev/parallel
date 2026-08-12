package runner

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/efureev/reggol"

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// E2E-харнесс базовой линии (docs/UPGRADE-SPEC.md, задача 0.5).
//
// Тестовый бинарник переиспользуется как «болтливый» дочерний процесс: это
// кроссплатформенно и не зависит от внешних утилит вроде seq или yes.
// Режим генератора включается флагом -e2e.gen.

const (
	e2eMarker      = "E2E"
	e2eEnd         = "END"
	e2eChains      = 8
	e2eDefaultRows = 500
	// e2eWidth заведомо больше PIPE_BUF (512 на macOS, 4096 на Linux):
	// именно на таких строках запись в один дескриптор из нескольких горутин
	// может разорвать строку — находка P4.
	e2eWidth = 700
)

var (
	genLines  = flag.Int("e2e.gen", 0, "internal: print N lines and exit")
	genTag    = flag.String("e2e.tag", "", "internal: tag for generated lines")
	genWidth  = flag.Int("e2e.width", 0, "internal: payload width in bytes")
	e2eRows   = flag.Int("e2e.rows", e2eDefaultRows, "e2e: строк на цепочку")
	e2eReport = flag.Bool("e2e.report", false, "e2e: печатать замер пропускной способности")
	e2eStrict = flag.Bool("e2e.strict", false, "e2e: требовать целостности и полноты вывода (фазы 3 и 4)")
)

func TestMain(m *testing.M) {
	flag.Parse()

	if *genLines > 0 {
		emitLines(*genTag, *genLines, *genWidth)
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// emitLines печатает lines помеченных строк заданной ширины и завершается.
func emitLines(tag string, lines, width int) {
	w := bufio.NewWriterSize(os.Stdout, 64*1024)
	payload := strings.Repeat("x", width)

	for i := range lines {
		_, _ = fmt.Fprintf(w, "%s %s %d %s %s\n", e2eMarker, tag, i, payload, e2eEnd)
	}

	_ = w.Flush()
}

// buildE2EFlow собирает цепочки, каждая из которых запускает генератор со своей меткой.
func buildE2EFlow(t *testing.T, chains, rows, width int) []*flow.CommandChain {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	result := make([]*flow.CommandChain, 0, chains)

	for c := range chains {
		tag := "chain" + strconv.Itoa(c)

		chain := &flow.CommandChain{Name: tag, Color: reggol.ColorFgCyan}
		chain.Add(flow.Command{
			Name: tag,
			Cmd:  self,
			Args: []string{
				"-e2e.gen=" + strconv.Itoa(rows),
				"-e2e.tag=" + tag,
				"-e2e.width=" + strconv.Itoa(width),
			},
			Pipe: true,
		})

		result = append(result, chain)
	}

	return result
}

// drainPipe читает всё, что попало в пайп, до его закрытия.
func drainPipe(r *os.File, dst *[]string, wg *sync.WaitGroup) {
	defer wg.Done()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for sc.Scan() {
		*dst = append(*dst, sc.Text())
	}
}

// TestE2EOutputIntegrity гоняет несколько болтливых цепочек параллельно и проверяет,
// что ни одна строка вывода не порвана, не склеена с соседней и не потеряна.
//
// Вывод направляется в настоящий os.Pipe, а не в bytes.Buffer: чересполосица
// возникает на уровне write(2), и на буфере в памяти не воспроизводится.
func TestE2EOutputIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: пропуск в коротком режиме")
	}

	rows := *e2eRows
	want := e2eChains * rows

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	var (
		collected []string
		wg        sync.WaitGroup
	)

	wg.Add(1)

	go drainPipe(pr, &collected, &wg)

	trans := ui.CreateTransformer()
	out := reggol.NewConsoleWriter(func(w *reggol.ConsoleWriter) {
		w.Out = pw
		w.Trans = trans
	})
	lgr := reggol.New(out)

	mgr := NewManager(&lgr)
	chains := buildE2EFlow(t, e2eChains, rows, e2eWidth)

	start := time.Now()

	if err := mgr.ExecuteParallel(context.Background(), chains); err != nil {
		t.Fatalf("ExecuteParallel: %v", err)
	}

	elapsed := time.Since(start)

	_ = pw.Close()
	wg.Wait()
	_ = pr.Close()

	st := inspectOutput(collected)

	if *e2eReport {
		reportStats(t, st, rows, want, elapsed)
	}

	// На фазе 0 тест снимает базовую линию, а не выносит приговор: оба дефекта
	// известны, зафиксированы в docs/AUDIT.md и закрываются фазами 3 и 4.
	// Флаг -e2e.strict превращает замеры в требования; по итогам фазы 4 он
	// становится режимом по умолчанию и уходит в CI.
	report := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)

		if *e2eStrict {
			t.Error(msg)

			return
		}

		t.Log("ИЗВЕСТНЫЙ ДЕФЕКТ: " + msg)
	}

	// Целостность строки: чересполосица при записи в один дескриптор (находка P4).
	if st.merged != 0 {
		report("склеенных строк %d, ожидалось 0", st.merged)
	}

	if st.intact != st.markers {
		report("порвано строк %d: целых %d при %d маркерах",
			st.markers-st.intact, st.intact, st.markers)
	}

	// Полнота вывода: часть строк не доходит вовсе — чтение пайпов прерывается
	// при выходе процесса (находка C10).
	if st.intact != want {
		report("дошло %d строк из %d (потеряно %d)", st.intact, want, want-st.intact)
	}
}

// reportStats печатает замер пропускной способности и разбивку по цепочкам.
func reportStats(t *testing.T, st outputStats, rows, want int, elapsed time.Duration) {
	t.Helper()

	t.Logf("цепочек=%d строк/цепочку=%d ширина=%d итого=%d за %v → %.0f строк/с",
		e2eChains, rows, e2eWidth, want, elapsed.Round(time.Millisecond),
		float64(want)/elapsed.Seconds())
	t.Logf("маркеров=%d целых=%d склеенных=%d уникальных=%d потеряно=%d (%.1f%%)",
		st.markers, st.intact, st.merged, st.unique, want-st.intact,
		float64(want-st.intact)/float64(want)*100)

	for c := range e2eChains {
		tag := "chain" + strconv.Itoa(c)
		t.Logf("  %s: дошло %d из %d", tag, st.perChain[tag], rows)
	}
}

// e2eLineRe разбирает сгенерированную строку: метка, цепочка, номер, тело, конец.
var e2eLineRe = regexp.MustCompile(e2eMarker + ` (chain\d+) (\d+) (x+) ` + e2eEnd)

// outputStats — разбор собранного вывода.
//
// markers — сколько всего меток встретилось; intact — сколько записей уцелело целиком;
// merged — сколько строк вывода содержат больше одной записи (потеря границы строки);
// unique — различные записи «цепочка/номер»; perChain — сколько записей дошло от каждой цепочки.
type outputStats struct {
	markers  int
	intact   int
	merged   int
	unique   int
	perChain map[string]int
}

// inspectOutput разбирает собранный вывод и считает статистику целостности.
func inspectOutput(lines []string) outputStats {
	st := outputStats{perChain: make(map[string]int)}
	unique := make(map[string]struct{})

	for _, line := range lines {
		count := strings.Count(line, e2eMarker)
		if count == 0 {
			continue
		}

		st.markers += count

		if count > 1 {
			st.merged++
		}

		for _, m := range e2eLineRe.FindAllStringSubmatch(line, -1) {
			if len(m[3]) != e2eWidth {
				continue
			}

			st.intact++
			st.perChain[m[1]]++
			unique[m[1]+"/"+m[2]] = struct{}{}
		}
	}

	st.unique = len(unique)

	return st
}
