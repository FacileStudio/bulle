package settings

import (
	"os"
	"path/filepath"
	"strconv"
)

var runUserRoot = "/run/user"

const xdgApp = "bulle"

// ConfigDir is where bulle keeps what a person edits: config.yml, profiles/
// and jobs/. It is $XDG_CONFIG_HOME/bulle, or ~/.config/bulle.
func ConfigDir() (string, error) {
	migrateLegacy()
	return configDir()
}

// DataDir is where bulle keeps what it accumulates and cannot rebuild: memory,
// sessions and the Matrix bot's keys. It is $XDG_DATA_HOME/bulle, or
// ~/.local/share/bulle.
func DataDir() (string, error) {
	migrateLegacy()
	return dataDir()
}

// StateDir is where bulle keeps what it can regenerate: the ledger, the run
// log, trust decisions and delivery logs. It is $XDG_STATE_HOME/bulle, or
// ~/.local/state/bulle.
func StateDir() (string, error) {
	migrateLegacy()
	return stateDir()
}

// RuntimeDir is where bulle keeps sockets and discovery files that must not
// outlive a login. It is $XDG_RUNTIME_DIR/bulle. A process that does not
// inherit the variable, cron above all, falls back to /run/user/<uid>/bulle
// when that login directory exists, so it finds the same sockets as the
// daemon; failing that, a private directory under the system temp dir.
func RuntimeDir() (string, error) {
	if dir := absoluteEnv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, xdgApp), nil
	}
	login := filepath.Join(runUserRoot, strconv.Itoa(os.Getuid()))
	if info, err := os.Stat(login); err == nil && info.IsDir() {
		return filepath.Join(login, xdgApp), nil
	}
	return filepath.Join(os.TempDir(), xdgApp+"-"+strconv.Itoa(os.Getuid())), nil
}

// UserConfigFile is the name of the user-level config inside ConfigDir.
const UserConfigFile = "config.yml"
