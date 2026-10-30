package agent

import (
	"strings"
	"testing"
)

func fakeCrontab(initial string, out *string) Crontab {
	return Crontab{
		Read:  func() (string, error) { return initial, nil },
		Write: func(s string) error { *out = s; return nil },
	}
}

func TestInstallCronEntriesWritesBlocks(t *testing.T) {
	var out string
	entries := []CronEntry{{Name: "assistant-watch", Schedule: "*/15 * * * *", Command: "/bin/bulle assistant watch"}}
	if err := InstallCronEntries(fakeCrontab("MAILTO=x\n", &out), entries, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MAILTO=x", "# BEGIN BULLE JOB assistant-watch", "*/15 * * * * /bin/bulle assistant watch", "# END BULLE JOB assistant-watch"} {
		if !strings.Contains(out, want) {
			t.Errorf("crontab missing %q:\n%s", want, out)
		}
	}
}

func TestInstallCronEntriesLegacy(t *testing.T) {
	legacy := "# BEGIN BULLE JOB assistant-pulse\n*/30 * * * * bulle cron run assistant-pulse\n# END BULLE JOB assistant-pulse\n"
	var out string
	entries := []CronEntry{{Name: "assistant-watch", Schedule: "*/15 * * * *", Command: "x"}}
	if err := InstallCronEntries(fakeCrontab(legacy, &out), entries, false); err == nil || out != "" {
		t.Fatalf("want refusal, got err=%v out=%q", err, out)
	}
	if err := InstallCronEntries(fakeCrontab(legacy, &out), entries, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "assistant-pulse") || !strings.Contains(out, "assistant-watch") {
		t.Errorf("legacy not replaced:\n%s", out)
	}
}
