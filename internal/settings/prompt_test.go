package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// additional_prompt is the one prompt setting that layers instead of
// replacing, so it travels the whole chain like every other one rather than
// being a second way to overwrite the base.
func TestAdditionalPromptFallsThroughTheWholeChain(t *testing.T) {
	written(t, "session:\n  additional_prompt: from-the-file\n")
	if config, err := settings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if config.Additional != "from-the-file" {
		t.Errorf("additional = %q, want the file's value", config.Additional)
	}

	t.Setenv(EnvPrefix+"ADDITIONAL_PROMPT", "from-the-environment")
	if config, _ := settings(Config{}); config.Additional != "from-the-environment" {
		t.Errorf("additional = %q, want the environment to win over the file", config.Additional)
	}

	flag := Config{Session: Session{Additional: "from-the-flag"}}
	if config, _ := settings(flag); config.Additional != "from-the-flag" {
		t.Errorf("additional = %q, want the flag to win over the environment", config.Additional)
	}
}

// A profile's persona is the same setting in a layer of its own, so it follows
// the same rule as the profile's other fields: it beats the file that named it,
// and the environment beats it. Only one persona ever reaches the prompt — the
// layers replace each other rather than stacking.
func TestAProfilePersonaBeatsTheFilesAndTheEnvironmentBeatsTheProfile(t *testing.T) {
	written(t, "profile: terse\nsession:\n  additional_prompt: from-the-file\n")
	clearEnv(t, "BULLE_PROFILE", "NACELLE_PROFILE")

	dir := filepath.Join(os.Getenv("HOME"), ".bulle", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	persona := "name: terse\nadditional_prompt: from-the-profile\n"
	if err := os.WriteFile(filepath.Join(dir, "terse.yml"), []byte(persona), 0o644); err != nil {
		t.Fatal(err)
	}

	if config, err := settings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if config.Additional != "from-the-profile" {
		t.Errorf("additional = %q, want the profile's persona over the file's", config.Additional)
	}

	t.Setenv(EnvPrefix+"ADDITIONAL_PROMPT", "from-the-environment")
	if config, _ := settings(Config{}); config.Additional != "from-the-environment" {
		t.Errorf("additional = %q, want the environment to win over the profile", config.Additional)
	}
}

// Replacing the base prompt and steering it are different acts, so the two
// settings have to hold at once: system_prompt overwrites as it always did,
// and the persona is still there to be appended after it.
func TestSystemPromptAndAdditionalPromptCompose(t *testing.T) {
	written(t, "session:\n  system_prompt: replacement base\n  additional_prompt: stay in character\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.System != "replacement base" {
		t.Errorf("system = %q, want system_prompt to replace the base as before", config.System)
	}
	if config.Additional != "stay in character" {
		t.Errorf("additional = %q, want the persona kept beside a replacement base", config.Additional)
	}
}
