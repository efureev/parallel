package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/efureev/parallel/internal/flow"
)

// escSeq — начало любой ANSI-последовательности.
const escSeq = "\x1b["

// TestNoANSIWhenNotTerminal — сторож против регрессии, ставшей возможной при
// переходе на reggol v1.
//
// В 0.x раскраска гасилась при перенаправлении вывода неявно: TextStyle.Wrap
// спрашивал глобальный флаг архивной зависимости gh.tarampamp.am/colors,
// который инициализировался через isatty. Зависимости больше нет, Wrap удалён,
// и решение о цвете принимаем мы сами. Если это решение потеряется, в файлы
// начнут попадать escape-последовательности.
func TestNoANSIWhenNotTerminal(t *testing.T) {
	t.Run("буфер в памяти", func(t *testing.T) {
		var buf bytes.Buffer

		out := NewOutput(&buf)
		lgr, formatter := out.Logger(), out.Formatter()

		chain := &flow.CommandChain{Name: "alpha", ColorIdx: 1}
		chain.Add(flow.Command{Name: "worker", Cmd: "echo", Args: []string{"hi"}})

		lgr.Info("info line", F("key", "value"))
		lgr.Blocks(formatter.FormatChainInfo(chain, chain.Commands()[0]).Header, "worker", "payload")
		lgr.Blocks(formatter.ChainPrefix(chain), "worker", "payload")
		lgr.Blocks(formatter.Divider(), "worker", "payload")

		if err := out.Close(); err != nil {
			t.Fatalf("close output: %v", err)
		}

		assertNoANSI(t, buf.String())
	})

	t.Run("обычный файл", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.log")

		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		out := NewOutput(f)
		lgr, formatter := out.Logger(), out.Formatter()

		chain := &flow.CommandChain{Name: "beta", ColorIdx: 2}
		chain.Add(flow.Command{Name: "worker", Cmd: "echo"})

		lgr.Info("into a file")
		lgr.Blocks(formatter.ChainPrefix(chain), "worker", "payload")

		if err := out.Close(); err != nil {
			t.Fatalf("close output: %v", err)
		}

		if err := f.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}

		assertNoANSI(t, string(data))
	})
}

// TestPaletteHonoursColorFlag фиксирует само правило: без раскраски текст
// возвращается как есть, с раскраской — обрамляется кодами.
func TestPaletteHonoursColorFlag(t *testing.T) {
	plain := NewPalette(false, false)
	if got := plain.Wrap(0, "text"); got != "text" {
		t.Errorf("без раскраски ожидался исходный текст, получено %q", got)
	}

	colored := NewPalette(false, true)

	got := colored.Wrap(0, "text")
	if !strings.Contains(got, escSeq) {
		t.Errorf("с раскраской ожидались ANSI-коды, получено %q", got)
	}

	if !strings.Contains(got, "text") {
		t.Errorf("текст потерялся при раскраске: %q", got)
	}
}

func assertNoANSI(t *testing.T, out string) {
	t.Helper()

	if out == "" {
		t.Fatal("вывод пуст — тест ничего не проверил")
	}

	if strings.Contains(out, escSeq) {
		t.Errorf("в неинтерактивный вывод попали ANSI-последовательности:\n%q", out)
	}
}
