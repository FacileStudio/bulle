package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	s "github.com/FacileStudio/bulle/internal/settings"
)

type testConfigEnv struct {
	file string
}

func setupConfigEnv(t *testing.T) testConfigEnv {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return testConfigEnv{file: filepath.Join(home, s.ConfigFile)}
}

func (e testConfigEnv) write(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(e.file, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func (e testConfigEnv) read(t *testing.T, over s.Config) s.Config {
	t.Helper()
	c, err := s.Settings("", over)
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	return c
}

// compact_at is unset by default: a session with no opinion of its own derives
// its ceiling from the context window and the compaction ratios, so a non-zero
// default would quietly make the ratio ladder dead. The constant survives as
// the fallback for a backend that reports no window at all.
func TestCompactAtDefaultsUnset(t *testing.T) {
	defaults := s.Defaults("")
	if defaults.CompactAt != nil {
		t.Fatalf("default compact_at = %d, want unset so the ratios decide", *defaults.CompactAt)
	}
	if s.DefaultCompactAt != 75_000 {
		t.Errorf("DefaultCompactAt = %d, want the 75000 windowless fallback", s.DefaultCompactAt)
	}
	env := setupConfigEnv(t)
	if c := env.read(t, s.Config{}); c.CompactAt != nil {
		t.Errorf("resolved compact_at = %d, want unset", *c.CompactAt)
	}
}

func TestCompactAtPrecedence(t *testing.T) {
	env := setupConfigEnv(t)
	env.write(t, "limits:\n  compact_at: 204800\n")
	if c := env.read(t, s.Config{}); *c.CompactAt != 204800 {
		t.Errorf("file compact_at = %d, want 204800", *c.CompactAt)
	}

	t.Setenv("BULLE_COMPACT_AT", "300000")
	if c := env.read(t, s.Config{}); *c.CompactAt != 300000 {
		t.Errorf("env compact_at = %d, want 300000", *c.CompactAt)
	}

	c := env.read(t, s.Config{Limits: s.Limits{CompactAt: new(int64(400000))}})
	if *c.CompactAt != 400000 {
		t.Errorf("flag compact_at = %d, want 400000", *c.CompactAt)
	}
}

func TestCompactAtFileVariants(t *testing.T) {
	env := setupConfigEnv(t)
	env.write(t, "provider:\n  backend: anthropic\n")
	if c := env.read(t, s.Config{}); c.CompactAt != nil {
		t.Errorf("unmentioned compact_at = %d, want unset", *c.CompactAt)
	}

	env.write(t, "limits:\n  compact_at: 0\n")
	if c := env.read(t, s.Config{}); c.CompactAt == nil || *c.CompactAt != 0 {
		t.Errorf("compact_at: 0 = %v, want an explicit 0 (compaction off)", c.CompactAt)
	}
}
