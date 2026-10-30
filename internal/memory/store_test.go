package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var noon = time.Date(2026, 10, 30, 12, 5, 0, 0, time.UTC)

type fakeModel struct {
	reply string
	err   error
}

func (f fakeModel) Complete(context.Context, string) (string, error) { return f.reply, f.err }

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAppendRefusesOverflow(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Append(MemoryFile, strings.Repeat("a", 2400)); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(MemoryFile, strings.Repeat("b", 200)); err != ErrOverCap {
		t.Fatalf("want ErrOverCap, got %v", err)
	}
	if got := read(t, s.path(MemoryFile)); strings.Contains(got, "b") {
		t.Fatal("overflowing write reached the file")
	}
}

func TestRememberReadBack(t *testing.T) {
	s := New(t.TempDir())
	line, err := s.Remember("  Yann  likes\ntea ", noon)
	if err != nil || line != "- [12:05] Yann likes tea" {
		t.Fatalf("line %q err %v", line, err)
	}
	if got := read(t, s.path("notes", "2026-10-30.md")); got != line+"\n" {
		t.Fatalf("file %q", got)
	}
	info, _ := os.Stat(s.path("notes", "2026-10-30.md"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
	if _, err := s.Remember("", noon); err != ErrEmptyFact {
		t.Fatal(err)
	}
	if _, err := s.Remember(strings.Repeat("x", 501), noon); err != ErrFactTooLong {
		t.Fatal(err)
	}
}

func TestForgetAcrossFilesWithBackup(t *testing.T) {
	s := New(t.TempDir())
	add(t, s, UserFile, "- Likes Coffee\n- Lives in Paris")
	add(t, s, MemoryFile, "- coffee at five")
	note(t, s, "drinks COFFEE daily", noon)
	n, err := s.Forget("coffee")
	if err != nil || n != 3 {
		t.Fatalf("removed %d err %v", n, err)
	}
	if got := read(t, s.path(UserFile)); got != "- Lives in Paris\n" {
		t.Fatalf("user %q", got)
	}
	backups, _ := filepath.Glob(s.path("backups", "*"))
	if len(backups) != 3 {
		t.Fatalf("backups %v", backups)
	}
}

func TestSnapshotFormat(t *testing.T) {
	s := New(t.TempDir())
	if s.Snapshot(noon) != "" {
		t.Fatal("empty memory must render nothing")
	}
	add(t, s, MemoryFile, strings.Repeat("m", 1474))
	add(t, s, UserFile, "- Yann")
	got := s.Snapshot(noon)
	if !strings.Contains(got, "MEMORY [58% 1,474/2,500 chars]") || !strings.Contains(got, delimiter) {
		t.Fatalf("snapshot %q", got)
	}
	if !strings.HasPrefix(got, "USER PROFILE [0% 6/1,500 chars]\n- Yann") {
		t.Fatalf("snapshot %q", got)
	}
}
