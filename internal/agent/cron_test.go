package agent

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestApplyJobDefaultsToNoShell(t *testing.T) {
	base := settings.Defaults("")
	tru := true

	job := settings.CronJob{
		Name:    "brief",
		Workdir: "~/x",
		Model:   "model-x",
	}
	cfg := applyJob(base, job)

	if *cfg.Bash != false {
		t.Errorf("commands not set defaults to bash=false, got true")
	}
	if *cfg.ApproveTools != false {
		t.Errorf("approval gate must be off for a scheduled run, got true")
	}
	if cfg.Root != expandHome("~/x") {
		t.Errorf("workdir not applied: got %q", cfg.Root)
	}
	if cfg.Model != "model-x" {
		t.Errorf("model not applied: got %q", cfg.Model)
	}
	if *cfg.Diffs != *base.Diffs {
		t.Errorf("unrelated toggle must be inherited untouched")
	}

	job.Commands = &tru
	if cfg2 := applyJob(base, job); *cfg2.Bash != true {
		t.Errorf("commands: true must enable bash, got false")
	}
}

func TestFindJobFile(t *testing.T) {
	files := []settings.JobFile{
		{Path: "/jobs/one.yml", Job: settings.CronJob{Name: "one"}},
		{Path: "/jobs/two.yml", Job: settings.CronJob{Name: "two"}},
	}
	if f, err := findJobFile(files, "two"); err != nil || f.Job.Name != "two" {
		t.Errorf("findJobFile(two) = %q, %v; want two, nil", f.Job.Name, err)
	}
	if _, err := findJobFile(files, "missing"); err == nil {
		t.Errorf("findJobFile(missing) should error")
	}
}

func TestJobPointerDefaults(t *testing.T) {
	job := settings.CronJob{}
	if job.IsEnabled() {
		t.Errorf("a job with no enabled setting must default to disabled")
	}
	if jobCommands(job) {
		t.Errorf("a job with no commands setting must default to shell-less")
	}
}

func TestValidateDelivery(t *testing.T) {
	for _, want := range []string{"", "file:~/logs", "file:"} {
		if err := validateDelivery(want); err != nil {
			t.Errorf("validateDelivery(%q) = %v, want nil", want, err)
		}
	}
	for _, bad := range []string{"webhook:x", "file", "~/logs"} {
		if err := validateDelivery(bad); err == nil {
			t.Errorf("validateDelivery(%q) passed, want a refusal of the unknown target", bad)
		}
	}
}

// hoistCases is the table behind TestHoistJSON: each entry is the argv to
// install, the leading flags and subcommand arguments to pass, and the rest
// value plus rewritten os.Args the call should produce.
var hoistCases = []struct {
	argv, prefix, sub, rest, want []string
}{
	{
		argv: []string{"bulle", "cron", "list", "--json"},
		sub:  []string{"list", "--json"},
		rest: []string{"list"},
		want: []string{"bulle", "--json", "cron", "list"},
	},
	{
		argv:   []string{"bulle", "-json", "cron", "list"},
		prefix: []string{"-json"},
		sub:    []string{"list"},
		rest:   []string{"list"},
		want:   []string{"bulle", "-json", "cron", "list"},
	},
	{
		argv: []string{"bulle", "cron", "list", "-json=false"},
		sub:  []string{"list", "-json=false"},
		rest: []string{"list"},
		want: []string{"bulle", "-json=false", "cron", "list"},
	},
	{
		argv: []string{"bulle", "cron", "run", "brief"},
		sub:  []string{"run", "brief"},
		rest: []string{"run", "brief"},
		want: []string{"bulle", "cron", "run", "brief"},
	},
}

func TestHoistJSON(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	for _, tc := range hoistCases {
		os.Args = tc.argv
		if rest := hoistJSON(tc.prefix, tc.sub); strings.Join(rest, " ") != strings.Join(tc.rest, " ") {
			t.Errorf("hoistJSON(%v, %v) rest = %v, want %v", tc.prefix, tc.sub, rest, tc.rest)
		}
		if got := strings.Join(os.Args, " "); got != strings.Join(tc.want, " ") {
			t.Errorf("hoistJSON(%v, %v) os.Args = %q, want %q", tc.prefix, tc.sub, got, tc.want)
		}
	}
}

func TestCronUsageErrors(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	for _, args := range [][]string{
		{"bulle", "cron", "run"},
		{"bulle", "cron", "install"},
		{"bulle", "cron", "uninstall"},
		{"bulle", "cron", "bogus"},
	} {
		os.Args = args
		handled, err := checkCronFlag()
		var usage *UsageError
		if !handled || err == nil || !errors.As(err, &usage) {
			t.Errorf("checkCronFlag(%v) = %t, %v; want a usage error", args, handled, err)
		}
	}
}

func TestCronJobsJSON(t *testing.T) {
	enabled, commands := true, false
	data, err := cronJobsJSON([]settings.CronJob{
		{Name: "brief", When: "daily", Enabled: &enabled, Commands: &commands, Workdir: "~/x", Delivery: "file:~/logs"},
	})
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	var jobs []cronJobJSON
	if err := json.Unmarshal(data, &jobs); err != nil {
		t.Fatalf("the document is not the job list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	job := jobs[0]
	if job.Name != "brief" || job.When != "daily" || !job.Enabled || job.Commands || job.Workdir != "~/x" || job.Delivery != "file:~/logs" {
		t.Errorf("job json = %+v, want the pointer settings resolved to their defaults", job)
	}
	empty, err := cronJobsJSON(nil)
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	if string(empty) != "[]" {
		t.Errorf("an empty job set marshals to %s, want []", empty)
	}
}
