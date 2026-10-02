package agent

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestCheckCronInstallable(t *testing.T) {
	tru := true
	job := settings.CronJob{Name: "daily-brief", Workdir: "~/x", Enabled: &tru}
	if err := checkCronInstallable(job); err != nil {
		t.Errorf("checkCronInstallable(valid job) = %v, want nil", err)
	}

	for _, bad := range []string{"", "has space", "sl/ash", `quo"te`, "café", "job$"} {
		job.Name = bad
		if err := checkCronInstallable(job); err == nil {
			t.Errorf("checkCronInstallable(name %q) = nil, want error", bad)
		}
	}

	job.Name = "daily-brief"
	job.Workdir = ""
	if err := checkCronInstallable(job); err == nil {
		t.Errorf("checkCronInstallable(no workdir) = nil, want error")
	}

	job.Workdir = "~/x"
	job.Enabled = nil
	if err := checkCronInstallable(job); err == nil {
		t.Errorf("checkCronInstallable(disabled) = nil, want error")
	}
}

func TestNormalizeCronSchedule(t *testing.T) {
	cases := []struct {
		input, want string
		err         bool
	}{
		{"daily", "0 0 * * *", false},
		{"@daily", "0 0 * * *", false},
		{"hourly", "0 * * * *", false},
		{"@hourly", "0 * * * *", false},
		{"0 9 * * *", "0 9 * * *", false},
		{"*/15 * * * *", "*/15 * * * *", false},
		{"invalid cron", "", true},
		{"* * * * * *", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeCronSchedule(tc.input)
		if tc.err && err == nil {
			t.Errorf("normalizeCronSchedule(%q) expected error, got nil", tc.input)
		}
		if !tc.err && err != nil {
			t.Errorf("normalizeCronSchedule(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("normalizeCronSchedule(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatCronBlock(t *testing.T) {
	block := formatCronBlock("news", "0 9 * * *", "/usr/bin/bulle", "/home/user/.bulle/logs")
	if !strings.Contains(block, "# BEGIN BULLE JOB news") {
		t.Errorf("expected begin marker in %s", block)
	}
	if !strings.Contains(block, "# END BULLE JOB news") {
		t.Errorf("expected end marker in %s", block)
	}
	if !strings.Contains(block, "0 9 * * * /usr/bin/bulle cron run news >> /home/user/.bulle/logs/news.log 2>&1") {
		t.Errorf("expected command line in %s", block)
	}
}
