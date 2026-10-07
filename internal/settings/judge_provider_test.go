package settings

import (
	"strings"
	"testing"
)

// Provider is a string, not a pointer, so the merge rule is the same one
// Model and BaseURL already follow: an empty value never clears one a lower
// layer supplied. A file that turns the judge on and names no provider keeps
// the default standing, and a file that names clef keeps it through an
// environment layer that says nothing.
func TestJudgeProviderMergesLikeModel(t *testing.T) {
	written(t, "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: clef\n")

	c, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.Provider != JudgeProviderClef {
		t.Errorf("judge provider = %q, want the file's %q", c.Compaction.Judge.Provider, JudgeProviderClef)
	}

	t.Setenv(EnvPrefix+"COMPACTION_JUDGE_PROVIDER", "jev")
	c, err = settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.Provider != JudgeProviderJEV {
		t.Errorf("judge provider = %q, want the environment to win", c.Compaction.Judge.Provider)
	}

	t.Setenv(EnvPrefix+"COMPACTION_JUDGE_PROVIDER", "")
	c, err = settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.Provider != JudgeProviderClef {
		t.Errorf("judge provider = %q, want the file's %q to survive an empty env var", c.Compaction.Judge.Provider, JudgeProviderClef)
	}
}

// The provider is read from the environment the same way Model and BaseURL
// are: through envGet, which falls back to the legacy NACELLE_ name and
// treats an unset variable as unmentioned.
func TestJudgeProviderComesFromTheEnvironment(t *testing.T) {
	written(t, "")
	t.Setenv(EnvPrefix+"COMPACTION_JUDGE_PROVIDER", JudgeProviderClef)

	c, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.Provider != JudgeProviderClef {
		t.Errorf("judge provider = %q, want the environment's %q", c.Compaction.Judge.Provider, JudgeProviderClef)
	}
}

// Provider is the one judge setting that decides which transport the history
// goes out on, so an unknown name is refused at load the same way an unusable
// ladder is: a config that builds a judge against a transport that does not
// exist sends history to an endpoint that answers nothing, and the failure
// arrives as a silent fallback rather than at the time it was written.
func TestJudgeRejectsAnUnknownProvider(t *testing.T) {
	tests := map[string]string{
		"a mistyped jev":           "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: jev1\n",
		"a mistyped clef":          "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: klu\n",
		"an OpenRouter model slug": "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: cloudflare/clef\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			written(t, body)
			_, err := settings(Config{})
			if err == nil {
				t.Error("settings accepted an unknown judge provider, want a load error naming both valid values")
				return
			}
			if !strings.Contains(err.Error(), "limits.compaction.judge.provider") {
				t.Errorf("error = %q, want it to name limits.compaction.judge.provider", err.Error())
			}
			if !strings.Contains(err.Error(), JudgeProviderJEV) || !strings.Contains(err.Error(), JudgeProviderClef) {
				t.Errorf("error = %q, want it to name both %q and %q", err.Error(), JudgeProviderJEV, JudgeProviderClef)
			}
		})
	}
}

// The judge's key follows the provider: clef is reached through OpenRouter and
// reads OPENROUTER_API_KEY, jev reads TYPESAFE_API_KEY, and each judge gets the
// vendor's own variable rather than the other's. The environment layer can only
// see its own provider, so ResolveKeys — where the merged provider is known —
// is what puts the key in step with a file that named the other one.
func TestJudgeKeyFollowsTheProviderVendor(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "sk-typesafe")
	t.Setenv("OPENROUTER_API_KEY", "sk-openrouter")
	written(t, "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: clef\n")

	cfg, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if err := ResolveKeys(&cfg); err != nil {
		t.Fatalf("ResolveKeys: %v", err)
	}
	if cfg.Compaction.Judge.APIKey != "sk-openrouter" {
		t.Errorf("judge key = %q, want OPENROUTER_API_KEY for the clef provider", cfg.Compaction.Judge.APIKey)
	}

	t.Setenv(EnvPrefix+"COMPACTION_JUDGE_PROVIDER", JudgeProviderJEV)
	cfg, err = settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if err := ResolveKeys(&cfg); err != nil {
		t.Fatalf("ResolveKeys: %v", err)
	}
	if cfg.Compaction.Judge.APIKey != "sk-typesafe" {
		t.Errorf("judge key = %q, want TYPESAFE_API_KEY for the jev provider", cfg.Compaction.Judge.APIKey)
	}
}

// An explicit model and base_url still win over a named provider: the escape
// hatch for pointing the judge at a proxy or a versioned build is not
// advertised in the scaffold, but it keeps working for files that already use
// it.
func TestJudgeProviderYieldsToAnExplicitModel(t *testing.T) {
	written(t, "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: clef\n      model: ~typesafe/jev-latest\n      base_url: https://openrouter.ai/api\n")

	c, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.Provider != JudgeProviderClef {
		t.Errorf("judge provider = %q, want the file's %q", c.Compaction.Judge.Provider, JudgeProviderClef)
	}
	if c.Compaction.Judge.Model != "~typesafe/jev-latest" {
		t.Errorf("judge model = %q, want the explicit value to win", c.Compaction.Judge.Model)
	}
	if c.Compaction.Judge.BaseURL != "https://openrouter.ai/api" {
		t.Errorf("judge base_url = %q, want the explicit value to win", c.Compaction.Judge.BaseURL)
	}
}
