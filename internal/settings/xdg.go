package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const xdgApp = "bulle"

// ConfigDir is where bulle keeps what a person edits: config.yml, profiles/
// and jobs/. It is $XDG_CONFIG_HOME/bulle, or ~/.config/bulle.
func ConfigDir() (string, error) {
	return xdgDir("XDG_CONFIG_HOME", ".config")
}

// DataDir is where bulle keeps what it accumulates and cannot rebuild: memory,
// sessions and the Matrix bot's keys. It is $XDG_DATA_HOME/bulle, or
// ~/.local/share/bulle.
func DataDir() (string, error) {
	return xdgDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

// StateDir is where bulle keeps what it can regenerate: the ledger, the run
// log, trust decisions and delivery logs. It is $XDG_STATE_HOME/bulle, or
// ~/.local/state/bulle.
func StateDir() (string, error) {
	return xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state"))
}

// RuntimeDir is where bulle keeps sockets and discovery files that must not
// outlive a login. It is $XDG_RUNTIME_DIR/bulle, or a private directory under
// the system temp dir when the variable is unset.
func RuntimeDir() (string, error) {
	if dir := absoluteEnv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, xdgApp), nil
	}
	return filepath.Join(os.TempDir(), xdgApp+"-"+strconv.Itoa(os.Getuid())), nil
}

// UserConfigFile is the name of the user-level config inside ConfigDir.
const UserConfigFile = "config.yml"

func xdgDir(env, fallback string) (string, error) {
	if dir := absoluteEnv(env); dir != "" {
		return filepath.Join(dir, xdgApp), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", env, err)
	}
	return filepath.Join(home, fallback, xdgApp), nil
}

func absoluteEnv(name string) string {
	dir := os.Getenv(name)
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	return dir
}
