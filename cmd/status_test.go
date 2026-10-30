package cmd

import (
	"bytes"
	"strings"
	"testing"

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
