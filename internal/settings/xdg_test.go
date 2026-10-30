package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

func TestXDGDirsHonourTheEnvironment(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(name, "")
	}
	cases := []struct {
		name string
		fn   func() (string, error)
		want string
	}{
		{"config", ConfigDir, filepath.Join(home, ".config", "bulle")},
		{"data", DataDir, filepath.Join(home, ".local", "share", "bulle")},
		{"state", StateDir, filepath.Join(home, ".local", "state", "bulle")},
	}
	for _, c := range cases {
		got, err := c.fn()
		if err != nil || got != c.want {
			t.Errorf("%s = %q, %v, want %q", c.name, got, err, c.want)
		}
	}
}

func TestXDGEnvOverridesAndRejectsRelativePaths(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)
	abs := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", abs)
	t.Setenv("XDG_DATA_HOME", "relative/path")
	t.Setenv("XDG_RUNTIME_DIR", abs)
	if got, _ := ConfigDir(); got != filepath.Join(abs, "bulle") {
		t.Errorf("config = %q", got)
	}
	if got, _ := DataDir(); got != filepath.Join(home, ".local", "share", "bulle") {
		t.Errorf("a relative XDG_DATA_HOME must be ignored, got %q", got)
	}
	if got, _ := RuntimeDir(); got != filepath.Join(abs, "bulle") {
		t.Errorf("runtime = %q", got)
	}
}

func TestRuntimeDirFallsBackToAPrivateTempDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got, err := RuntimeDir()
	if err != nil || filepath.Base(got) == "" || filepath.Dir(got) != filepath.Clean(os.TempDir()) {
		t.Fatalf("runtime = %q, %v", got, err)
	}
}
