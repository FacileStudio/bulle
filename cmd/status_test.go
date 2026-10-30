package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestPrintStatusReportsFailure(t *testing.T) {
	var buf bytes.Buffer
	lines := []statusLine{{statusOK, "a", "fine"}, {statusWarn, "b", "meh"}}
	if printStatus(&buf, lines) {
		t.Fatal("ok and warn lines must not fail the status")
	}
	if !printStatus(&buf, append(lines, statusLine{statusFail, "c", "broken"})) {
		t.Fatal("a FAIL line must fail the status")
	}
	if !strings.Contains(buf.String(), "FAIL  c") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestJobLineFlagsAnEnabledJobThatIsNotInstalled(t *testing.T) {
	enabled := true
	job := settings.CronJob{Name: "x", When: "daily", Enabled: &enabled}
	got := jobLine(settings.JobFile{Path: t.TempDir() + "/x.yml", Raw: []byte("a"), Job: job}, "")
	if got.level == statusOK {
		t.Errorf("an untrusted enabled job must not be ok: %+v", got)
	}
	job.Enabled = nil
	if got := jobLine(settings.JobFile{Job: job}, ""); got.level != statusOK {
		t.Errorf("a disabled job is fine: %+v", got)
	}
}

func TestWatchLineDoesNotFailDuringQuietHours(t *testing.T) {
	old := time.Now().Add(-6 * time.Hour)
	if got := watchLine(old, true); got.level != statusOK {
		t.Errorf("quiet hours must not fail: %+v", got)
	}
	if got := watchLine(old, false); got.level != statusFail {
		t.Errorf("a stale watch outside quiet hours must fail: %+v", got)
	}
	if got := watchLine(time.Time{}, false); got.level != statusWarn {
		t.Errorf("a watch that never ran is a warning: %+v", got)
	}
}

func TestWatchLineNeverRanHasNoTimestamp(t *testing.T) {
	got := watchLine(time.Time{}, true)
	if got.level != statusWarn || strings.Contains(got.detail, "0001") {
		t.Errorf("watchLine = %+v", got)
	}
}

func TestMissingLogDirsFindsARedirectThatCannotOpen(t *testing.T) {
	good := t.TempDir()
	crontab := "# comment bulle >> /nonexistent/x.log\n" +
		"*/15 * * * * /bin/bulle assistant watch >> " + good + "/w.log 2>&1\n" +
		"0 9 * * * /bin/bulle cron run news >> /nonexistent/dir/n.log 2>&1\n"
	got := missingLogDirs(crontab)
	if len(got) != 1 || got[0] != "/nonexistent/dir" {
		t.Errorf("missing = %v", got)
	}
}
