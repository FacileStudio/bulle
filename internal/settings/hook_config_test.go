package settings

import "testing"

func TestShowHooksAndOutputDefaultOn(t *testing.T) {
	fallback := defaults()
	if !*fallback.ShowHooks {
		t.Error("show_hooks = false, want it on by default")
	}
	if !*fallback.ShowHookOutput {
		t.Error("show_hook_output = false, want it on by default")
	}
}

func TestShowHooksAndOutputFromFile(t *testing.T) {
	written(t, "ui:\n  show_hooks: false\n  show_hook_output: false\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.ShowHooks {
		t.Error("show_hooks = true, want the file's false to win")
	}
	if *config.ShowHookOutput {
		t.Error("show_hook_output = true, want the file's false to win")
	}
}

func TestHookOutputAliasFromFile(t *testing.T) {
	written(t, "ui:\n  hook_output: false\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.HookOutput {
		t.Error("hook_output = true, want the file's false to win")
	}
}

// The environment beats the defaults. It runs on a home of the test's own like
// its siblings do: without that this read the developer's real ~/.bulle.yml and
// profiles, so an unrelated key in someone's own config could fail the suite.
func TestShowHooksFromEnv(t *testing.T) {
	written(t, "")
	t.Setenv(EnvPrefix+"SHOW_HOOKS", "false")
	t.Setenv(EnvPrefix+"SHOW_HOOK_OUTPUT", "false")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.ShowHooks {
		t.Error("show_hooks = true, want environment's false to win")
	}
	if *config.ShowHookOutput {
		t.Error("show_hook_output = true, want environment's false to win")
	}
}
