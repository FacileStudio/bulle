package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func add(t *testing.T, s *Store, name, line string) {
	t.Helper()
	if err := s.Append(name, line); err != nil {
		t.Fatal(err)
	}
}

func note(t *testing.T, s *Store, fact string, at time.Time) {
	t.Helper()
	if _, err := s.Remember(fact, at); err != nil {
		t.Fatal(err)
	}
}

func seeded(t *testing.T) *Store {
	t.Helper()
	s := New(t.TempDir())
	add(t, s, MemoryFile, "- old stale line\n- keeps")
	note(t, s, "moved to Lyon", noon.AddDate(0, 0, -1))
	return s
}

func TestConsolidateAppliesAndBacksUp(t *testing.T) {
	s := seeded(t)
	before := read(t, s.path(MemoryFile))
	m := fakeModel{reply: `{"add":["Lives in Lyon","lives in lyon","keeps"],"drop":["- old stale line","not an existing line"]}`}
	rep, err := s.Consolidate(context.Background(), m, noon, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 || rep.Added[0] != "- Lives in Lyon" || len(rep.Dropped) != 1 {
		t.Fatalf("report %+v", rep)
	}
	if got := read(t, s.path(MemoryFile)); got != "- keeps\n- Lives in Lyon\n" {
		t.Fatalf("memory %q", got)
	}
	if got := read(t, s.path("backups", "MEMORY.md.2026-10-30")); got != before {
		t.Fatalf("backup %q", got)
	}
	if err := s.Revert("2026-10-30"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s.path(MemoryFile)); got != before {
		t.Fatalf("revert %q", got)
	}
}

func TestConsolidateBadJSONChangesNothing(t *testing.T) {
	s := seeded(t)
	before := read(t, s.path(MemoryFile))
	for _, bad := range []string{"sure! here you go", `{"add": "x"}`, `{"add":[]} trailing`} {
		if _, err := s.Consolidate(context.Background(), fakeModel{reply: bad}, noon, Options{}); err == nil {
			t.Fatalf("no error for %q", bad)
		}
	}
	if read(t, s.path(MemoryFile)) != before {
		t.Fatal("memory changed")
	}
	if _, err := s.Consolidate(context.Background(), fakeModel{err: context.Canceled}, noon, Options{}); err == nil {
		t.Fatal("model error swallowed")
	}
}

func TestConsolidateRefusesOverCap(t *testing.T) {
	s := seeded(t)
	before := read(t, s.path(MemoryFile))
	big, _ := json.Marshal(map[string][]string{"add": {strings.Repeat("z", 450), strings.Repeat("y", 450), strings.Repeat("x", 450), strings.Repeat("w", 450), strings.Repeat("v", 450), strings.Repeat("u", 450)}})
	rep, err := s.Consolidate(context.Background(), fakeModel{reply: string(big)}, noon, Options{})
	if err != nil || len(rep.Refused) != 6 || len(rep.Added) != 0 {
		t.Fatalf("report %+v err %v", rep, err)
	}
	if read(t, s.path(MemoryFile)) != before {
		t.Fatal("memory changed")
	}
}

func TestRevertValidatesDate(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Revert("../x"); err == nil {
		t.Fatal("bad date accepted")
	}
	if err := s.Revert("2026-01-01"); err == nil {
		t.Fatal("missing backup accepted")
	}
}

func TestRememberToolReturnsLine(t *testing.T) {
	s := New(t.TempDir())
	tool := RememberTool(s, func() time.Time { return noon })
	out, err := tool.Run(context.Background(), json.RawMessage(`{"fact":"likes tea"}`))
	if err != nil || out != "- [12:05] likes tea" {
		t.Fatalf("out %q err %v", out, err)
	}
	if _, err := tool.Run(context.Background(), json.RawMessage(`{"fact":""}`)); err == nil {
		t.Fatal("empty fact accepted")
	}
}
