package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

func seed(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacySplitsTheOldHomeAcrossTheXDGDirs(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)
	legacy := filepath.Join(home, ".bulle")
	for _, rel := range []string{"jobs/a.yml", "profiles/p.yml", "memory/USER.md", "sessions/s.jsonl", "chat/matrix.db", "trust.json", "logs/n.log", "state/ledger.json"} {
		seed(t, filepath.Join(legacy, rel))
	}
	seed(t, filepath.Join(home, ".bulle.yml"))
	seed(t, filepath.Join(home, ".local", "state", "bulle", "cron.jsonl"))

	want := []string{
		".config/bulle/config.yml", ".config/bulle/jobs/a.yml", ".config/bulle/profiles/p.yml",
		".local/share/bulle/memory/USER.md", ".local/share/bulle/sessions/s.jsonl", ".local/share/bulle/chat/matrix.db",
		".local/state/bulle/trust.json", ".local/state/bulle/logs/n.log", ".local/state/bulle/ledger.json",
		".local/state/bulle/cron.jsonl",
	}
	for range 2 {
		migrateLegacy()
		for _, rel := range want {
			if _, err := os.Stat(filepath.Join(home, rel)); err != nil {
				t.Errorf("%s missing: %v", rel, err)
			}
		}
	}
	for _, gone := range []string{".bulle", ".bulle.yml"} {
		if _, err := os.Stat(filepath.Join(home, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should be gone, stat err = %v", gone, err)
		}
	}
}

func TestMigrateLegacyNeverOverwrites(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)
	seed(t, filepath.Join(home, ".bulle", "trust.json"))
	dest := filepath.Join(home, ".local", "state", "bulle", "trust.json")
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	migrateLegacy()
	if got, _ := os.ReadFile(dest); string(got) != "new" {
		t.Errorf("destination overwritten: %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".bulle", "trust.json")); err != nil {
		t.Errorf("source must stay when the destination exists: %v", err)
	}
}

func TestMigrateLegacyKeepsARelativeSymlinkWorking(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)
	seed(t, filepath.Join(home, "dotfiles", "profiles", "p.yml"))
	if err := os.MkdirAll(filepath.Join(home, ".bulle"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../dotfiles/profiles", filepath.Join(home, ".bulle", "profiles")); err != nil {
		t.Fatal(err)
	}
	migrateLegacy()
	if _, err := os.Stat(filepath.Join(home, ".config", "bulle", "profiles", "p.yml")); err != nil {
		t.Errorf("the moved symlink dangles: %v", err)
	}
}

func TestMigrateLegacyClearsRuntimeLeftovers(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".bulle", "ide"), 0o700); err != nil {
		t.Fatal(err)
	}
	seed(t, filepath.Join(home, ".bulle", "trust.json"))
	migrateLegacy()
	if _, err := os.Stat(filepath.Join(home, ".bulle")); !os.IsNotExist(err) {
		t.Errorf("an empty ide/ must not keep ~/.bulle alive: %v", err)
	}
}
