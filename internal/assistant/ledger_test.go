package assistant

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestLedger(t *testing.T) (*Ledger, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "ledger.json")
	l, err := NewLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	return l, path
}

func landingPage(status string) Item {
	return Item{
		Source: SourceTask,
		ID:     "42",
		Title:  "create landing page",
		Status: status,
		Due:    time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
	}
}

func TestLedgerReplaysLandingPageIncident(t *testing.T) {
	l, _ := newTestLedger(t)
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	item := landingPage("in-progress")
	if !l.ShouldSurface(item, now) {
		t.Fatal("new task must surface")
	}
	if err := l.MarkSurfaced([]Item{item}, now); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 10; i++ {
		if l.ShouldSurface(item, now.Add(time.Duration(i)*15*time.Minute)) {
			t.Fatalf("evaluation %d surfaced an unchanged item", i)
		}
	}
	if !l.ShouldSurface(landingPage("in-review"), now.Add(3*time.Hour)) {
		t.Fatal("status change must surface again")
	}
}

func TestLedgerThresholds(t *testing.T) {
	l, _ := newTestLedger(t)
	start := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)
	ev := Item{Source: SourceCalendar, ID: "e1", When: start}
	early := start.Add(-3 * time.Hour)
	if err := l.MarkSurfaced([]Item{ev}, early); err != nil {
		t.Fatal(err)
	}
	if l.ShouldSurface(ev, early.Add(time.Hour)) {
		t.Fatal("event far away must stay silent")
	}
	soon := start.Add(-45 * time.Minute)
	if !l.ShouldSurface(ev, soon) {
		t.Fatal("event within 60 minutes must surface")
	}
	if err := l.MarkSurfaced([]Item{ev}, soon); err != nil {
		t.Fatal(err)
	}
	if l.ShouldSurface(ev, soon.Add(10*time.Minute)) {
		t.Fatal("threshold must fire once")
	}
	task := Item{Source: SourceTask, ID: "t1", Due: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}
	day := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	if err := l.MarkSurfaced([]Item{task}, day); err != nil {
		t.Fatal(err)
	}
	if !l.ShouldSurface(task, day.Add(24*time.Hour)) {
		t.Fatal("task due today must surface")
	}
}

func TestLedgerSnoozeAndNoise(t *testing.T) {
	l, _ := newTestLedger(t)
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	item := landingPage("todo")
	if err := l.Snooze(item.Key(), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if l.ShouldSurface(item, now) {
		t.Fatal("snoozed item surfaced")
	}
	if !l.ShouldSurface(item, now.Add(2*time.Hour)) {
		t.Fatal("snooze did not expire")
	}
	mail := Item{Source: SourceMail, ID: "t9"}
	if err := l.MarkNoise(mail.Key()); err != nil {
		t.Fatal(err)
	}
	if l.ShouldSurface(mail, now.Add(48*time.Hour)) {
		t.Fatal("noise surfaced")
	}
}

func TestSnoozeUntilTomorrow(t *testing.T) {
	l, _ := newTestLedger(t)
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no tzdata")
	}
	now := time.Date(2026, 10, 30, 15, 0, 0, 0, loc)
	if err := l.SnoozeUntilTomorrow("task:1", now, loc); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 31, 9, 0, 0, 0, loc)
	if got := l.entries["task:1"].SnoozedUntil; !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := nextNineAM(time.Date(2026, 10, 31, 7, 0, 0, 0, loc), loc); !got.Equal(want) {
		t.Fatalf("before 9h: got %v", got)
	}
}

func TestLedgerSurvivesCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "ledger.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := NewLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("corrupt copy missing: %v", err)
	}
	now := time.Now()
	if err := l.MarkSurfaced([]Item{landingPage("a")}, now); err != nil {
		t.Fatal(err)
	}
	again, err := NewLedger(path)
	if err != nil || again.ShouldSurface(landingPage("a"), now) {
		t.Fatalf("reload failed: %v", err)
	}
	assertMode(t, path, 0o600)
	assertMode(t, filepath.Dir(path), 0o700)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != want {
		t.Fatalf("%s: mode %v err %v", path, info, err)
	}
}

func TestLedgerPrune(t *testing.T) {
	l, _ := newTestLedger(t)
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	old, fresh := landingPage("a"), Item{Source: SourceTask, ID: "9"}
	if err := l.Seen([]Item{old}, now.Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := l.Seen([]Item{fresh}, now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := l.Prune(now); err != nil {
		t.Fatal(err)
	}
	if _, ok := l.entries[old.Key()]; ok {
		t.Fatal("old entry kept")
	}
	if _, ok := l.entries[fresh.Key()]; !ok {
		t.Fatal("fresh entry pruned")
	}
}

func TestLedgerConcurrentMarkSurfaced(t *testing.T) {
	l, path := newTestLedger(t)
	now := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			it := Item{Source: SourceMail, ID: string(rune('a' + i))}
			if err := l.MarkSurfaced([]Item{it}, now); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	again, err := NewLedger(path)
	if err != nil || len(again.entries) != 20 {
		t.Fatalf("entries %d err %v", len(again.entries), err)
	}
}
