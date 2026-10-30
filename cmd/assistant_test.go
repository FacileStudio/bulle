package cmd

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/settings"
)

func TestAssistantInstallWritesExpectedLines(t *testing.T) {
	var written string
	ct := agent.Crontab{
		Read:  func() (string, error) { return "", nil },
		Write: func(s string) error { written = s; return nil },
	}
	cfg := settings.Assistant{Digests: map[string]string{"morning": "08:30", "midday": "14:00", "evening": "20:00"}}
	if err := installAssistantEntries(ct, cfg, "/bin/bulle", "/logs", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"*/15 * * * * /bin/bulle assistant watch >> /logs/assistant-watch.log 2>&1",
		"30 8 * * * /bin/bulle assistant digest morning >> /logs/assistant-digest-morning.log 2>&1",
		"0 14 * * * /bin/bulle assistant digest midday",
		"0 20 * * * /bin/bulle assistant digest evening",
		"30 3 * * * /bin/bulle assistant consolidate",
		"# BEGIN BULLE JOB assistant-consolidate",
	} {
		if !strings.Contains(written, want) {
			t.Errorf("crontab missing %q:\n%s", want, written)
		}
	}
}

func TestAssistantInstallWritesTimezoneFirstAndQuotesPaths(t *testing.T) {
	var written string
	ct := agent.Crontab{
		Read:  func() (string, error) { return "CRON_TZ=old\n", nil },
		Write: func(s string) error { written = s; return nil },
	}
	cfg := settings.Assistant{Timezone: "Europe/Paris"}
	if err := installAssistantEntries(ct, cfg, "/opt/my bulle/100%/bulle", "/logs", false); err != nil {
		t.Fatal(err)
	}
	tz := strings.Index(written, "CRON_TZ=Europe/Paris")
	watch := strings.Index(written, "*/15")
	if tz < 0 || watch < 0 || tz > watch {
		t.Fatalf("CRON_TZ must precede entries:\n%s", written)
	}
	if !strings.Contains(written, `'/opt/my bulle/100\%/bulle' assistant watch`) {
		t.Fatalf("path not quoted:\n%s", written)
	}
}

func TestAssistantInstallRefusesLegacy(t *testing.T) {
	legacy := "# BEGIN BULLE JOB assistant-checkin\n0 20 * * * bulle cron run assistant-checkin\n# END BULLE JOB assistant-checkin\n"
	ct := agent.Crontab{
		Read:  func() (string, error) { return legacy, nil },
		Write: func(string) error { t.Fatal("must not write"); return nil },
	}
	if err := installAssistantEntries(ct, settings.Assistant{}, "/bin/bulle", "/logs", false); err == nil {
		t.Fatal("want refusal")
	}
}
