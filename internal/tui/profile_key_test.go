package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/nacelle"
)

// Switching profiles mid-session reads the profile file rather than the resolved
// config, so it is the one place a profile's api_key_command has to be run a
// second time — the session's own start is the other. A profile that names a
// command instead of a key would otherwise switch to a backend built with no
// credential at all.
func TestProfileSwitchResolvesAKeyFromItsCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	profiles := filepath.Join(dir, ".bulle", "profiles")
	if err := os.MkdirAll(profiles, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "name: fromcmd\nprovider:\n  backend: openai\n  model: gpt-4o\n  api_key_command: \"echo sk-from-profile-command\"\n"
	if err := os.WriteFile(filepath.Join(profiles, "fromcmd.yml"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Model{}
	m.activeBackend = "anthropic"
	m.activeModel = "claude-opus-5"
	m.delegate = nacelle.Config{System: "test"}

	m.modelCmd("fromcmd")

	if m.activeAPIKey != "sk-from-profile-command" {
		t.Error("the active key is not what the profile's command printed")
	}
	if m.activeBackend != "openai" {
		t.Errorf("active backend = %q, want the profile's openai", m.activeBackend)
	}
}
