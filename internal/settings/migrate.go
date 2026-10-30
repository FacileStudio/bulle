package settings

import (
	"os"
	"path/filepath"
	"sync"
)

const (
	legacyHomeDir    = ".bulle"
	legacyConfigFile = ".bulle.yml"
	nacelleHomeDir   = ".nacelle"
	nacelleConfig    = ".nacelle.yml"
)

var legacyHomeEntries = []struct {
	name string
	to   func() (string, error)
	sub  string
}{
	{"jobs", configDir, "jobs"},
	{"profiles", configDir, "profiles"},
	{"memory", dataDir, "memory"},
	{"sessions", dataDir, "sessions"},
	{"chat", dataDir, "chat"},
	{"trust.json", stateDir, "trust.json"},
	{"hooks.json", stateDir, "hooks.json"},
	{"logs", stateDir, "logs"},
}

// ConfigPath is the user config file, ConfigDir/config.yml, or "" when no
// home directory resolves.
func ConfigPath() string {
	dir, err := ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, UserConfigFile)
}

// migrated records the home directories already migrated by this process, so
// a move that fails is reported once instead of on every path lookup.
var migrated sync.Map

// migrateLegacy carries an install from an older layout to the XDG one:
// ~/.nacelle and ~/.nacelle.yml to ~/.bulle and ~/.bulle.yml, then ~/.bulle
// and ~/.config/bulle/config.yml into the config, data and state directories. It never
// overwrites: an entry whose destination already exists stays where it was,
// and ~/.bulle is removed only once it is empty. Idempotent, and a fresh
// install has none of these paths.
func migrateLegacy() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	if _, done := migrated.LoadOrStore(home, true); done {
		return
	}
	renameIfMissing(filepath.Join(home, nacelleHomeDir), filepath.Join(home, legacyHomeDir))
	renameIfMissing(filepath.Join(home, nacelleConfig), filepath.Join(home, legacyConfigFile))
	if config, err := configDir(); err == nil {
		renameIfMissing(filepath.Join(home, legacyConfigFile), filepath.Join(config, UserConfigFile))
	}
	legacy := filepath.Join(home, legacyHomeDir)
	if missing(legacy) {
		return
	}
	for _, entry := range legacyHomeEntries {
		if base, err := entry.to(); err == nil {
			renameIfMissing(filepath.Join(legacy, entry.name), filepath.Join(base, entry.sub))
		}
	}
	if state, err := stateDir(); err == nil {
		mergeInto(filepath.Join(legacy, "state"), state)
	}
	dropRuntimeLeftovers(legacy)
	removeIfEmpty(legacy)
}
