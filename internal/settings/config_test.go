package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// written puts a config file in a home directory of the test's own, so a test
// never reads or writes the real one.
func written(t *testing.T, body string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(home, ConfigFile), []byte(body), 0o600); err != nil {
		t.Fatalf("writing the config: %v", err)
	}
}

func settings(flags Config) (Config, error) {
	return Settings("", flags)
}

// Most people never write one, so a missing file has to be ordinary rather
// than an error.
func TestNoConfigFileLeavesTheDefaultsStanding(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.Backend != "anthropic" || config.Root != "." {
		t.Errorf("config = %+v, want the defaults", config)
	}
}

// A config that cannot be parsed must not be skipped in silence: the setting
// you carefully wrote is simply not in effect, and nothing says so.
func TestAMalformedConfigIsAnError(t *testing.T) {
	written(t, "provider:\n  backend: [this is not a string")

	if _, err := settings(Config{}); err == nil {
		t.Fatal("a malformed config was accepted")
	}
}

// The mode setting defaults to tui, is set by the file, overridden by the
// environment, and beaten by the flag — the same linear chain as every other
// setting.
func TestModeFallsThroughTheWholeChain(t *testing.T) {
	written(t, "ui:\n  rendering_mode: tui\n")
	if config, _ := settings(Config{}); *config.Mode != "tui" {
		t.Errorf("mode = %q, want tui by default", *config.Mode)
	}

	written(t, "ui:\n  rendering_mode: inline\n")
	if config, _ := settings(Config{}); *config.Mode != "inline" {
		t.Errorf("mode = %q, want the file's inline", *config.Mode)
	}

	t.Setenv(EnvPrefix+"MODE", "inline")
	if config, _ := settings(Config{}); *config.Mode != "inline" {
		t.Errorf("mode = %q, want the environment to win over the file", *config.Mode)
	}

	flagMode := "tui"
	if config, _ := settings(Config{UI: UI{Mode: &flagMode}}); *config.Mode != "tui" {
		t.Errorf("mode = %q, want the flag to win over the environment", *config.Mode)
	}
}

func TestTheFileBeatsTheDefaults(t *testing.T) {
	written(t, "provider:\n  backend: openrouter\n  model: deepseek/deepseek-v4-flash-0731\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.Backend != "openrouter" || config.Model != "deepseek/deepseek-v4-flash-0731" {
		t.Errorf("config = %+v, want the file's backend and model", config)
	}
}

// Environment variables are overrides, not a separate mode. A sibling CLI in
// this suite read its variables only inside a branch that ignored the config
// file, which turned what its README called overrides into two mutually
// exclusive modes nobody could tell apart.
func TestTheEnvironmentBeatsTheFileWithoutReplacingIt(t *testing.T) {
	written(t, "provider:\n  backend: openrouter\n  model: from-the-file\nsession:\n  root: /from/the/file\n")
	t.Setenv(EnvPrefix+"MODEL", "from-the-environment")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.Model != "from-the-environment" {
		t.Errorf("model = %q, want the environment to win", config.Model)
	}
	if config.Root != "/from/the/file" {
		t.Errorf("root = %q, want the file's value to survive an unrelated override", config.Root)
	}
}

func TestAFlagBeatsEverything(t *testing.T) {
	written(t, "provider:\n  model: from-the-file\n")
	t.Setenv(EnvPrefix+"MODEL", "from-the-environment")

	config, err := settings(Config{Provider: Provider{Model: "from-the-flag"}})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.Model != "from-the-flag" {
		t.Errorf("model = %q, want the flag to win", config.Model)
	}
}

// The reason every toggle is a pointer. A layer that says nothing and a layer
// that says false are different answers, and a bool cannot tell them apart —
// so a file turning something off has to survive a default that had it on.
func TestATurnedOffToggleIsNotMistakenForAnUnsetOne(t *testing.T) {
	written(t, "limits:\n  max_iterations: 3\nreasoning:\n  thinking: true\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.MaxIterations != 3 {
		t.Errorf("max iterations = %d, want the file's 3", *config.MaxIterations)
	}
	if !*config.Thinking {
		t.Error("thinking = false, want the file to have turned it on")
	}
	if !*config.Bash {
		t.Error("bash = false, want bash default on when no layer mentions it")
	}
}

// A value strconv cannot read means the writer meant something; falling through
// to the layer below is closer to that than silently choosing false.
func TestAnUnreadableEnvironmentValueFallsThroughRatherThanMeaningFalse(t *testing.T) {
	written(t, "tools:\n  run_command: true\n")
	t.Setenv(EnvPrefix+"BASH", "yes-please")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Bash {
		t.Error("bash = false, want file true to survive an unreadable override")
	}
}

func TestPromptPlaceholderComesFromTheFile(t *testing.T) {
	written(t, "ui:\n  prompt_placeholder: ask away\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.PromptPlaceholder != "ask away" {
		t.Errorf("prompt placeholder = %q, want the file's value", *config.PromptPlaceholder)
	}
}

func TestStartMessageComesFromTheFileAndDefaultsEmpty(t *testing.T) {
	written(t, "")
	if config, err := settings(Config{}); err != nil {
		t.Fatalf("settings: %v", err)
	} else if *config.StartMessage != *Defaults("").StartMessage {
		t.Errorf("start message = %q, want the defaults' art", *config.StartMessage)
	}

	written(t, "ui:\n  start_message: |\n    welcome\n    to bulle\n")
	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.StartMessage != "welcome\nto bulle\n" {
		t.Errorf("start message = %q, want the file's multiline block verbatim", *config.StartMessage)
	}
}

// The whole promise of refusing a malformed file, applied to the keys. A file
// saying max_iteration — one letter short — parsed cleanly, left the ceiling at
// 40 and cost real money on the next long run without a word about it.
func TestATypoInAKeyIsRefusedRatherThanIgnored(t *testing.T) {
	written(t, "max_iteration: 3\n")

	config, err := settings(Config{})
	if err == nil {
		t.Fatalf("settings = %+v, want a misspelt key refused", config)
	}
	if !strings.Contains(err.Error(), "max_iteration") {
		t.Errorf("error = %v, want it to name the key it did not know", err)
	}
}

// Under systemd, cron or `env -i` there is no HOME and no config file either,
// which is the ordinary case this client already handles. Refusing to start
// meant bulle could not run with every setting passed on the command line,
// for want of a file it was never going to read.
func TestAnUnresolvableHomeMeansNoConfigFileRatherThanNoProgram(t *testing.T) {
	t.Setenv("HOME", "")

	config, err := settings(Config{Provider: Provider{Model: "from-the-flag"}})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.Model != "from-the-flag" {
		t.Errorf("model = %q, want the flag honoured with nowhere to read a file from", config.Model)
	}
}
