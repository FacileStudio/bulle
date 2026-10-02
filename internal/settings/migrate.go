package settings

import (
	"os"
	"path/filepath"
)

// HomeDir is ~/.bulle, migrated from a legacy ~/.nacelle when only that
// exists, so an install from before the rename keeps its trust, jobs,
// sessions and MCP config.
func HomeDir() (string, error) {
	migrateLegacyHome()
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bulle"), nil
}

// migrateLegacyHome moves ~/.nacelle to ~/.bulle on first use of the new
// install. Idempotent: a fresh install has neither path.
func migrateLegacyHome() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	if !missing(filepath.Join(home, ".bulle")) || missing(filepath.Join(home, ".nacelle")) {
		return
	}
	if err := os.Rename(filepath.Join(home, ".nacelle"), filepath.Join(home, ".bulle")); err != nil {
		return
	}
}

// migrateLegacyConfig moves ~/.nacelle.yml to ~/.bulle.yml when only the
// legacy file exists, so settings written before the rename carry over.
func migrateLegacyConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	if !missing(filepath.Join(home, ConfigFile)) || missing(filepath.Join(home, ".nacelle.yml")) {
		return
	}
	if err := os.Rename(filepath.Join(home, ".nacelle.yml"), filepath.Join(home, ConfigFile)); err != nil {
		return
	}
}

// missing reports whether a path does not exist.
func missing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// ConfigPath is where the config file lives (HOME).
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ConfigFile)
}
