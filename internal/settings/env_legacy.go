package settings

import (
	"os"
	"strings"
)

// envGet reads a setting's variable under EnvPrefix, falling back to the
// legacy NACELLE_ name.
func envGet(name string) string {
	v, _ := lookup(EnvPrefix + name)
	return v
}

// lookup reads a variable, falling back to the legacy NACELLE_ name when the
// BULLE_ one is unset.
func lookup(name string) (string, bool) {
	if v, ok := os.LookupEnv(name); ok {
		return v, ok
	}
	if rest, ok := strings.CutPrefix(name, EnvPrefix); ok {
		return os.LookupEnv(legacyEnvPrefix + rest)
	}
	return os.LookupEnv(name)
}
