package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

// resolve is Settings as the real callers reach it: the defaults are the base the
// resolver applies itself, and the overlay carries only what the command line
// typed (FromFlags returns a sparse Config, never a filled-in one).
func resolve(t *testing.T) settings.Config {
	t.Helper()
	cfg, err := settings.Settings("", settings.Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	return cfg
}

// writeHome puts a config file in a home of the test's own, so nothing here reads
// or writes the real one, and clears the two variables that could turn the judge
// on behind the file's back.
func writeHome(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv(settings.EnvPrefix+"COMPACTION_JUDGE", "")
	if err := os.WriteFile(filepath.Join(home, settings.ConfigFile), []byte(body), 0o600); err != nil {
		t.Fatalf("writing the config: %v", err)
	}
}

// The judge is optional: a config that never mentions it builds none, which is
// what keeps a session that did not ask for one off the network.
func TestJudgeIsOffUntilTheConfigTurnsItOn(t *testing.T) {
	writeHome(t, "provider:\n  backend: anthropic\n")

	if judge := Judge(resolve(t).Compaction); judge != nil {
		t.Errorf("judge = %v, want nil while the config leaves it off", judge)
	}
}

// And it is enableable from that file alone: one key in ~/.bulle.yml, with no flag
// and no environment variable, is all it takes to switch it on and have the
// session carry it.
func TestJudgeIsBuiltFromTheConfigFile(t *testing.T) {
	writeHome(t, "limits:\n  compaction:\n    soft_ratio: 0.5\n    judge:\n      enabled: true\n      model: jev-test\n")

	cfg := resolve(t)
	if judge := Judge(cfg.Compaction); judge == nil {
		t.Fatal("judge = nil, want the file's enabled: true to build one")
	}
	if cfg.Compaction.Judge.Model != "jev-test" {
		t.Errorf("judge model = %q, want the file's own", cfg.Compaction.Judge.Model)
	}
	if soft, _ := cfg.Compaction.Ratios(); soft != 0.5 {
		t.Errorf("soft ratio = %v, want the file's 0.5 alongside it", soft)
	}

	budget := ResolveBudget(cfg.CompactAt, cfg.Compaction, &fixedWindow{window: 200_000})
	if CompactionConfig(budget, cfg.Compaction).Judge == nil {
		t.Error("the session's compaction config carries no judge, so the file's opt-in never reaches a pass")
	}
}
