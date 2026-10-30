package memory

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgetRefusesShortQuery(t *testing.T) {
	s := New(t.TempDir())
	add(t, s, MemoryFile, "- a tea line\n- other")
	for _, q := range []string{"a", "tea", " ab "} {
		if _, err := s.Forget(q); !errors.Is(err, ErrShortQuery) {
			t.Fatalf("%q: err %v", q, err)
		}
	}
	if got := read(t, s.path(MemoryFile)); !strings.Contains(got, "a tea line") {
		t.Fatalf("file changed: %q", got)
	}
}

func TestForgetKeepsBlankLinesAndRestores(t *testing.T) {
	s := New(t.TempDir())
	body := "# Notes\n\n- coffee at five\n\n- keeps\n"
	if err := s.writeAtomic(s.path(MemoryFile), []byte(body)); err != nil {
		t.Fatal(err)
	}
	res, err := s.ForgetReport("coffee")
	if err != nil || res.Removed != 1 || len(res.Backups) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if got := read(t, s.path(MemoryFile)); got != "# Notes\n\n\n- keeps\n" {
		t.Fatalf("blank lines changed: %q", got)
	}
	if err := s.Restore(res.Backups[0]); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s.path(MemoryFile)); got != body {
		t.Fatalf("restore: %q", got)
	}
}

func TestRevertWritesSafetyBackup(t *testing.T) {
	s := seeded(t)
	if _, err := s.Consolidate(context.Background(), fakeModel{reply: `{"add":["new fact"],"drop":[]}`}, noon, Options{}); err != nil {
		t.Fatal(err)
	}
	current := read(t, s.path(MemoryFile))
	if err := s.Revert("2026-10-30"); err != nil {
		t.Fatal(err)
	}
	pre, _ := filepath.Glob(s.path("backups", "MEMORY.md.pre-revert.*"))
	if len(pre) != 1 || read(t, pre[0]) != current {
		t.Fatalf("safety backup %v", pre)
	}
}

func TestConsolidateCapsDrops(t *testing.T) {
	s := New(t.TempDir())
	add(t, s, MemoryFile, "- a1\n- a2\n- a3\n- a4\n- a5")
	note(t, s, "something", noon.AddDate(0, 0, -1))
	m := fakeModel{reply: `{"add":[],"drop":["- a1","- a2","- a3","- a4","- a5"]}`}
	rep, err := s.Consolidate(context.Background(), m, noon, Options{})
	if err != nil || len(rep.Dropped) != MaxDropsPerRun {
		t.Fatalf("dropped %v err %v", rep.Dropped, err)
	}
}
