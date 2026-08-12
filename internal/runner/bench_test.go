package runner

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

const benchLineWidth = 80

// benchPayload строит вывод из lines строк шириной width байт.
func benchPayload(lines, width int) []byte {
	var buf bytes.Buffer

	buf.Grow(lines * (width + 1))

	for i := range lines {
		prefix := "line " + strconv.Itoa(i) + " "
		buf.WriteString(prefix)

		if pad := width - len(prefix); pad > 0 {
			buf.WriteString(strings.Repeat("x", pad))
		}

		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

// BenchmarkIndentBlock мерит форматирование блочного вывода не-pipe команды.
// Находка P3: конкатенация в цикле даёт квадратичный рост по аллокациям.
func BenchmarkIndentBlock(b *testing.B) {
	for _, lines := range []int{100, 1000, 10000} {
		b.Run(strconv.Itoa(lines), func(b *testing.B) {
			payload := benchPayload(lines, benchLineWidth)

			b.ReportAllocs()

			for b.Loop() {
				_ = indentBlock(payload)
			}
		})
	}
}
