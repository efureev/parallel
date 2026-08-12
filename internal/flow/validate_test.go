package flow

import (
	"path/filepath"
	"testing"
)

func TestMissingDirs(t *testing.T) {
	existing := t.TempDir()
	missing := filepath.Join(existing, "nope")

	chain := &CommandChain{Name: "build"}
	chain.Add(Command{Name: "ok", Cmd: "echo", Dir: existing})
	chain.Add(Command{Name: "no-dir", Cmd: "echo"})
	chain.Add(Command{Name: "broken", Cmd: "echo", Dir: missing})

	f := &Flow{}
	f.AddChain(chain)

	got := MissingDirs(f)

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 missing dir, got %d: %+v", len(got), got)
	}

	if got[0].Chain != "build" || got[0].Command != "broken" || got[0].Dir != missing {
		t.Errorf("unexpected entry: %+v", got[0])
	}
}

func TestMissingDirs_NilFlow(t *testing.T) {
	if got := MissingDirs(nil); got != nil {
		t.Fatalf("expected nil for nil flow, got %+v", got)
	}
}
