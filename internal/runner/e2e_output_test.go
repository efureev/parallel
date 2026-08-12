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

	"github.com/efureev/parallel/internal/flow"
	"github.com/efureev/parallel/internal/ui"
)

// E2E-проверка целостности вывода.
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
	// может разорвать строку.
	e2eWidth = 700
)

var (
	genLines  = flag.Int("e2e.gen", 0, "internal: print N lines and exit")
	genTag    = flag.String("e2e.tag", "", "internal: tag for generated lines")
	genWidth  = flag.Int("e2e.width", 0, "internal: payload width in bytes")
	e2eRows   = flag.Int("e2e.rows", e2eDefaultRows, "e2e: строк на цепочку")
	e2eReport = flag.Bool("e2e.report", false, "e2e: печатать замер пропускной способности")
	// Строгость включена по умолчанию: тест сторожит целостность вывода.
	// Флаг оставлен, чтобы можно было снять замеры на заведомо сломанном
	// дереве, не правя код.
	e2eLenient = flag.Bool("e2e.lenient", false, "e2e: только сообщать о дефектах вывода, не заваливать тест")
	// Режим дочернего процесса для проверки мягкого завершения на Windows.
	// Флаг объявлен здесь, а тест — в файле с тегом windows: флаги должны
	// разбираться на всех платформах, иначе дочерний процесс упадёт на неизвестном аргументе.
	genGraceful = flag.Bool("e2e.graceful-child", false, "internal: ждать консольного события и выйти штатно")
)

func TestMain(m *testing.M) {
	flag.Parse()

	if *genLines > 0 {
		emitLines(*genTag, *genLines, *genWidth)
		os.Exit(0)
	}

	if *genGraceful {
		runGracefulChild()
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

		chain := &flow.CommandChain{Name: tag, ColorIdx: c}
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
	requireIntegration(t)

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

	// Логгер строится портом: пакет runner про библиотеку логирования не знает.
	out := ui.NewOutput(pw)
	mgr := NewManager(out.Logger(), out.Formatter())
	chains := buildE2EFlow(t, e2eChains, rows, e2eWidth)

	start := time.Now()

	if err := mgr.ExecuteParallel(context.Background(), chains); err != nil {
		t.Fatalf("ExecuteParallel: %v", err)
	}

	elapsed := time.Since(start)

	// Досбрасываем буфер до закрытия пайпа, иначе хвост вывода потеряется.
	if err := out.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	_ = pw.Close()
	wg.Wait()
	_ = pr.Close()

	st := inspectOutput(collected)

	if *e2eReport {
		reportStats(t, st, rows, want, elapsed)
	}

	report := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)

		if *e2eLenient {
			t.Log("ИЗВЕСТНЫЙ ДЕФЕКТ: " + msg)

			return
		}

		t.Error(msg)
	}

	// Целостность строки: чересполосица при записи в один дескриптор.
	if st.merged != 0 {
		report("склеенных строк %d, ожидалось 0", st.merged)
	}

	if st.interleaved != 0 {
		report("наложено записей %d, ожидалось 0", st.interleaved)
	}

	// Обрыв записи на середине — след того же дефекта, что и потеря строк:
	// чтение пайпа прекращается, не дочитав текущую строку.
	if st.truncated != 0 {
		report("обрезано записей %d (обрыв чтения при выходе процесса)", st.truncated)
	}

	// Полнота вывода: часть строк не доходит вовсе — чтение пайпов прерывается
	// при выходе процесса.
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
	t.Logf("маркеров=%d целых=%d склеенных=%d обрезано=%d наложено=%d уникальных=%d потеряно=%d (%.1f%%)",
		st.markers, st.intact, st.merged, st.truncated, st.interleaved, st.unique, want-st.intact,
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
	markers int
	intact  int
	merged  int
	// truncated — записи, оборванные на середине: метка есть, завершителя нет.
	// Это след обрезания вывода при выходе процесса (C10), а не наложения записей.
	truncated int
	// interleaved — записи, испорченные наложением чужой записи: и метка, и
	// завершитель на месте, но содержимое не совпадает с ожидаемым (P4).
	interleaved int
	unique      int
	perChain    map[string]int
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

		matched := 0

		for _, m := range e2eLineRe.FindAllStringSubmatch(line, -1) {
			if len(m[3]) != e2eWidth {
				continue
			}

			matched++
			st.intact++
			st.perChain[m[1]]++
			unique[m[1]+"/"+m[2]] = struct{}{}
		}

		// Классифицируем испорченные записи: обрыв на середине — это след C10,
		// а наложение — след P4. Лечатся они в разных фазах, и смешивать их нельзя.
		if broken := count - matched; broken > 0 {
			if strings.HasSuffix(line, e2eEnd) {
				st.interleaved += broken
			} else {
				st.truncated += broken
			}
		}
	}

	st.unique = len(unique)

	return st
}
