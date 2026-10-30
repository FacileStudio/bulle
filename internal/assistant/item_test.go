package assistant

import (
	"testing"
	"time"
)

func TestFingerprintIgnoresTitleButNotStatus(t *testing.T) {
	due := time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC)
	a := Item{Source: SourceTask, ID: "7", Title: "landing", Status: "in-progress", Due: due}
	b := a
	b.Title = "landing page"
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("a title edit must not change the fingerprint")
	}
	b.Status = "in-review"
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("a status change must change the fingerprint")
	}
	if a.Key() != "task:7" {
		t.Fatalf("key = %q", a.Key())
	}
}
