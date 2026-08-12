package buildinfo

import (
	"strings"
	"testing"
)

func TestLong(t *testing.T) {
	out := Long()

	if out == "" {
		t.Fatal("Long must not be empty")
	}

	for _, want := range []string{"Version:", "Commit:", "Built at:", "Go:", "OS/Arch:", ProjectURL} {
		if !strings.Contains(out, want) {
			t.Errorf("Long output missing %q:\n%s", want, out)
		}
	}
}

func TestFallback(t *testing.T) {
	if got := fallback("", "n/a"); got != "n/a" {
		t.Errorf("fallback(\"\") = %q, want %q", got, "n/a")
	}

	if got := fallback("abc", "n/a"); got != "abc" {
		t.Errorf("fallback(\"abc\") = %q, want %q", got, "abc")
	}
}
