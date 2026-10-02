package agent

import (
	"strings"
	"testing"
)

func TestApplyCrontabBlockToEmpty(t *testing.T) {
	block := "# BEGIN BULLE JOB test\n0 0 * * * /bin/bulle cron run test >> /logs/test.log 2>&1\n# END BULLE JOB test"
	got := applyCrontabBlock("", "test", block)
	want := block + "\n"
	if got != want {
		t.Errorf("applyCrontabBlock(\"\") = %q, want %q", got, want)
	}
}

func TestApplyCrontabBlockReplaceExisting(t *testing.T) {
	initial := "# BEGIN BULLE JOB test\n0 0 * * * /bin/bulle cron run test >> /logs/test.log 2>&1\n# END BULLE JOB test\n"
	newBlock := "# BEGIN BULLE JOB test\n0 9 * * * /bin/bulle cron run test >> /logs/test.log 2>&1\n# END BULLE JOB test"
	got := applyCrontabBlock(initial, "test", newBlock)
	want := newBlock + "\n"
	if got != want {
		t.Errorf("applyCrontabBlock replace = %q, want %q", got, want)
	}
}

func TestApplyCrontabBlockPreservesOtherEntries(t *testing.T) {
	initial := "0 * * * * /bin/other\n\n# BEGIN BULLE JOB other\n0 0 * * * /bin/bulle cron run other\n# END BULLE JOB other\n"
	newBlock := "# BEGIN BULLE JOB test\n0 9 * * * /bin/bulle cron run test >> /logs/test.log 2>&1\n# END BULLE JOB test"
	got := applyCrontabBlock(initial, "test", newBlock)
	if !strings.Contains(got, "0 * * * * /bin/other") {
		t.Errorf("expected preserved other entry in %q", got)
	}
	if !strings.Contains(got, "# BEGIN BULLE JOB other") {
		t.Errorf("expected preserved other job block in %q", got)
	}
	if !strings.Contains(got, "# BEGIN BULLE JOB test") {
		t.Errorf("expected new test job block in %q", got)
	}
}

func TestStripCrontabBlock(t *testing.T) {
	initial := "# BEGIN BULLE JOB test\n0 0 * * * /bin/bulle cron run test\n# END BULLE JOB test\n0 * * * * /bin/other\n"
	got, removed := stripCrontabBlock(initial, "test")
	if !removed {
		t.Errorf("expected removed = true")
	}
	if strings.Contains(got, "BEGIN BULLE JOB test") {
		t.Errorf("expected block removed, got %q", got)
	}
	if !strings.Contains(got, "0 * * * * /bin/other") {
		t.Errorf("expected other entry preserved, got %q", got)
	}
}

func TestStripLegacyUntaggedLine(t *testing.T) {
	initial := "0 0 * * * /home/user/.local/bin/bulle cron run news\n0 1 * * * /bin/other\n"
	got, removed := stripCrontabBlock(initial, "news")
	if !removed {
		t.Errorf("expected removed = true")
	}
	if strings.Contains(got, "cron run news") {
		t.Errorf("expected legacy line removed, got %q", got)
	}
	if !strings.Contains(got, "0 1 * * * /bin/other") {
		t.Errorf("expected other entry preserved, got %q", got)
	}
}

func TestStripLegacyUntaggedLineDoesNotMatchPrefix(t *testing.T) {
	initial := "0 0 * * * /home/user/.local/bin/bulle cron run news-alerts\n0 1 * * * /bin/other\n"
	got, removed := stripCrontabBlock(initial, "news")
	if removed {
		t.Errorf("expected removed = false for news-alerts when stripping news")
	}
	if !strings.Contains(got, "cron run news-alerts") {
		t.Errorf("expected news-alerts preserved, got %q", got)
	}
}
