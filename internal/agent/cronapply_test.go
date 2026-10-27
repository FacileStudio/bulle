package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestApplyJobOverridesEachGroup(t *testing.T) {
	base := settings.Defaults("")
	bash := true
	fetch := false
	job := settings.CronJob{
		Provider:  settings.Provider{Backend: "openai", Model: "gpt-x", BaseURL: "https://api.example.com", APIKey: "k"},
		Security:  settings.Security{PathIsolation: &fetch, DenyElevation: &fetch, EnvIsolation: &fetch},
		Toggles:   settings.Toggles{Bash: &bash, Fetch: &fetch, Tasks: &fetch, SearchContent: &fetch, FindFiles: &fetch},
		Reasoning: settings.Reasoning{Effort: "high", Thinking: &fetch},
		Limits:    settings.Limits{MaxIterations: &iterationsOverride, CompactAt: &compactOverride},
	}
	cfg := applyJob(base, job)
	if cfg.Backend != "openai" || cfg.Model != "gpt-x" || cfg.BaseURL != "https://api.example.com" || cfg.APIKey != "k" {
		t.Errorf("provider overrides not applied: %+v", cfg.Provider)
	}
	if *cfg.PathIsolation || *cfg.DenyElevation || *cfg.EnvIsolation {
		t.Errorf("security overrides not applied")
	}
	if !*cfg.Bash || *cfg.Fetch || *cfg.Tasks || *cfg.SearchContent || *cfg.FindFiles {
		t.Errorf("tool overrides not applied")
	}
	if cfg.Effort != "high" || *cfg.Thinking {
		t.Errorf("reasoning overrides not applied")
	}
	if *cfg.MaxIterations != 5 || *cfg.CompactAt != 1000 {
		t.Errorf("limit overrides not applied")
	}
	if *cfg.ApproveTools {
		t.Errorf("a job cannot arm the approval gate")
	}
}

func TestApplyJobEmptyMembersFallBack(t *testing.T) {
	base := settings.Defaults("")
	job := settings.CronJob{}
	cfg := applyJob(base, job)
	if cfg.Backend != base.Backend || cfg.Model != base.Model || cfg.Effort != base.Effort {
		t.Errorf("an empty job must inherit the config: %+v", cfg.Provider)
	}
	if cfg.Gates != nil {
		t.Errorf("an empty job must not add gates")
	}
}

func TestApplyJobGroupModelBeatsFlatModel(t *testing.T) {
	job := settings.CronJob{Model: "flat", Provider: settings.Provider{Model: "group"}}
	if cfg := applyJob(settings.Defaults(""), job); cfg.Model != "group" {
		t.Errorf("provider.model must beat model, got %q", cfg.Model)
	}
	job = settings.CronJob{Model: "flat"}
	if cfg := applyJob(settings.Defaults(""), job); cfg.Model != "flat" {
		t.Errorf("flat model must still work, got %q", cfg.Model)
	}
}

func TestApplyJobCommandsVsTools(t *testing.T) {
	base := settings.Defaults("")
	off := false
	job := settings.CronJob{Commands: new(true), Toggles: settings.Toggles{Bash: &off}}
	if cfg := applyJob(base, job); *cfg.Bash {
		t.Errorf("tools.run_command must beat commands: true, got bash on")
	}
	job = settings.CronJob{Commands: new(true)}
	if cfg := applyJob(base, job); !*cfg.Bash {
		t.Errorf("commands: true must still enable bash")
	}
	job = settings.CronJob{}
	if cfg := applyJob(base, job); *cfg.Bash {
		t.Errorf("cron defaults to shell-less")
	}
}

func TestApplyJobAppendsGates(t *testing.T) {
	base := settings.Defaults("")
	job := settings.CronJob{Gates: []settings.GateSpec{{Name: "fmt", Command: []string{"true"}}}}
	cfg := applyJob(base, job)
	if len(cfg.Gates) != 1 || cfg.Gates[0].Name != "fmt" {
		t.Errorf("job gates must append to the config's, got %+v", cfg.Gates)
	}
}

func TestJobHooksBuildAndRefuseBadSpecs(t *testing.T) {
	hooks, err := jobHooks(settings.CronJob{Hooks: []settings.HookSpec{{
		On: "before_tool_call", Match: []string{"run_command"}, Run: "true", Timeout: "1s",
	}}})
	if err != nil || len(hooks) == 0 {
		t.Errorf("a valid job hook spec must build: %v, %v", hooks, err)
	}
	if _, err := jobHooks(settings.CronJob{Hooks: []settings.HookSpec{{On: "nope", Run: "true"}}}); err == nil {
		t.Errorf("an unbuildable hook spec must be refused before the run starts")
	}
	if hooks, err := jobHooks(settings.CronJob{}); hooks != nil || err != nil {
		t.Errorf("a hook-less job must build no hooks: %v, %v", hooks, err)
	}
}

func TestTrustCronJobAsksAndRecords(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := settings.JobsDir()
	if err := mkJobDir(dir); err != nil {
		t.Fatal(err)
	}
	path := dir + "/brief.yml"
	if err := writeFile(path, "name: brief\nprompt: say hi\n"); err != nil {
		t.Fatal(err)
	}
	if err := trustCronJob("brief", strings.NewReader("n\n")); err == nil {
		t.Errorf("a no answer must leave the job untrusted")
	}
	if err := trustCronJob("brief", strings.NewReader("y\n")); err != nil {
		t.Fatalf("trusting: %v", err)
	}
	assertTrusted(t, "brief")
}

func TestTrustReArmsAfterAnEdit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := settings.JobsDir()
	if err := mkJobDir(dir); err != nil {
		t.Fatal(err)
	}
	path := dir + "/brief.yml"
	if err := writeFile(path, "name: brief\nprompt: say hi\n"); err != nil {
		t.Fatal(err)
	}
	if err := trustCronJob("brief", strings.NewReader("yes\n")); err != nil {
		t.Fatalf("trusting: %v", err)
	}
	assertTrusted(t, "brief")
	if err := writeFile(path, "name: brief\nprompt: say hi\nwhen: daily\n"); err != nil {
		t.Fatal(err)
	}
	_, files, err := loadCronState()
	if err != nil {
		t.Fatal(err)
	}
	f, err := findJobFile(files, "brief")
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureTrusted(f); err == nil {
		t.Errorf("changed content must re-arm the trust gate")
	}
}

// A job file's compaction block merges over the resolved config after the job's
// own check has run, and that one validates the job's ladder filled with defaults
// rather than the ladder the merge will produce. A job that is usable alone can
// therefore combine with the session's ratios into one that cannot tier, and
// nothing downstream re-reads it. The merge is re-validated for that reason.
func TestAJobLadderIsValidatedAfterMerging(t *testing.T) {
	soft, smart, jobSmart, sane := 0.85, 0.9, 0.8, 0.88
	base := settings.Defaults("")
	base.Compaction.SoftRatio = &soft
	base.Compaction.SmartRatio = &smart

	_, err := jobConfig(base, settings.CronJob{Limits: settings.Limits{
		Compaction: settings.Compaction{SmartRatio: &jobSmart},
	}})
	if err == nil {
		t.Fatal("a ladder that cannot tier once merged must be refused")
	}
	if !strings.Contains(err.Error(), "limits.compaction") {
		t.Errorf("error = %q, want it to name the ladder", err)
	}

	if _, err := jobConfig(base, settings.CronJob{Limits: settings.Limits{
		Compaction: settings.Compaction{SmartRatio: &sane},
	}}); err != nil {
		t.Errorf("a merged ladder that can tier must load: %v", err)
	}
}

func assertTrusted(t *testing.T, name string) {
	t.Helper()
	_, files, err := loadCronState()
	if err != nil {
		t.Fatal(err)
	}
	f, err := findJobFile(files, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureTrusted(f); err != nil {
		t.Errorf("trusted job must pass the gate: %v", err)
	}
}

func mkJobDir(dir string) error { return os.MkdirAll(dir, 0o755) }

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }

func TestJobConfigAppliesProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	pdir := filepath.Join(home, ".bulle", "profiles")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	free := "name: free\nprovider:\n  backend: freellmapi\n  model: deepseek-chat\n"
	if err := os.WriteFile(filepath.Join(pdir, "free.yml"), []byte(free), 0o644); err != nil {
		t.Fatal(err)
	}
	base := settings.Defaults("")
	job := settings.CronJob{Profile: "free"}
	cfg, err := jobConfig(base, job)
	if err != nil {
		t.Fatalf("jobConfig: %v", err)
	}
	if cfg.Backend != "freellmapi" || cfg.Model != "deepseek-chat" {
		t.Errorf("profile not applied: got backend=%q model=%q", cfg.Backend, cfg.Model)
	}
	badJob := settings.CronJob{Profile: "nonexistent"}
	if _, err := jobConfig(base, badJob); err == nil {
		t.Error("expected error for nonexistent profile, got nil")
	}
}

func TestFormatJobPrompt(t *testing.T) {
	cases := []struct {
		now        time.Time
		wantPrefix string
	}{
		{time.Date(2026, 10, 27, 16, 30, 0, 0, time.FixedZone("CET", 3600)), "Current local time: 2026-10-27 16:30:00 CET (UTC+01:00)\n\n"},
		{time.Date(2026, 10, 27, 12, 0, 0, 0, time.FixedZone("NST", -12600)), "Current local time: 2026-10-27 12:00:00 NST (UTC-03:30)\n\n"},
	}
	for _, tc := range cases {
		res := formatJobPrompt("check tasks", tc.now)
		if !strings.HasPrefix(res, tc.wantPrefix) || !strings.HasSuffix(res, "check tasks") {
			t.Fatalf("formatJobPrompt = %q, want prefix %q and suffix %q", res, tc.wantPrefix, "check tasks")
		}
	}
}

var iterationsOverride = 5
var compactOverride int64 = 1000
