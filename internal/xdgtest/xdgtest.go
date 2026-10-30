// Package xdgtest isolates a test from the real XDG directories, so a test
// that touches bulle's paths can never write into the developer's own home.
package xdgtest

import (
	"os"
	"path/filepath"
	"testing"
)

var vars = []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR"}

// Isolate points HOME at home and clears every XDG variable for the test, so
// the default locations resolve under home.
func Isolate(t testing.TB, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	for _, name := range vars {
		t.Setenv(name, "")
	}
}

// ConfigPath returns the user config file under home, creating its directory.
func ConfigPath(t testing.TB, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "bulle")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "config.yml")
}
