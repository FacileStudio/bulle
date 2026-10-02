package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesFromDirectory(t *testing.T) {
	dir := t.TempDir()
	fastYml := `name: fast
provider:
  backend: google
  model: gemini-2.5-flash
reasoning:
  effort: low
`
	smartYml := `provider:
  backend: anthropic
  model: claude-opus-5
`
	if err := os.WriteFile(filepath.Join(dir, "fast.yml"), []byte(fastYml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "smart.yml"), []byte(smartYml), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles, err := LoadProfiles(dir)
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2", len(profiles))
	}
	if profiles[0].Name != "fast" || profiles[0].Provider.Backend != "google" {
		t.Errorf("profiles[0] = %+v, want fast/google", profiles[0])
	}
	if profiles[1].Name != "smart" || profiles[1].Provider.Backend != "anthropic" {
		t.Errorf("profiles[1] = %+v, want smart/anthropic", profiles[1])
	}
}

func TestLoadProfilesDuplicateName(t *testing.T) {
	dir := t.TempDir()
	p1 := "name: dup\nprovider:\n  backend: openai\n"
	p2 := "name: dup\nprovider:\n  backend: google\n"
	if err := os.WriteFile(filepath.Join(dir, "a.yml"), []byte(p1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte(p2), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProfiles(dir)
	if err == nil {
		t.Fatal("expected error on duplicate profile name, got nil")
	}
}

func TestApplyProfile(t *testing.T) {
	thinking := true
	budget := int64(4096)
	p := Profile{
		Name: "test",
		Provider: Provider{
			Backend: "openrouter",
			Model:   "deepseek/deepseek-r1",
			BaseURL: "https://openrouter.ai/api/v1",
		},
		Reasoning: Reasoning{
			Effort:   "high",
			Thinking: &thinking,
			Budget:   &budget,
		},
	}

	var c Config
	ApplyProfile(&c, p)

	if c.Backend != "openrouter" || c.Model != "deepseek/deepseek-r1" {
		t.Errorf("ApplyProfile didn't set backend/model: %+v", c)
	}
	if c.Effort != "high" || *c.Budget != 4096 {
		t.Errorf("ApplyProfile didn't set reasoning: %+v", c)
	}
}

// A profile carries the model-aware limits, because the same iteration ceiling
// and compaction threshold are wrong for a fast small model and a slow large
// one — tuning that belongs with the identity, not with the checkout.
func TestApplyProfileCarriesLimitsAndPersona(t *testing.T) {
	iterations, compactAt := 12, int64(150_000)
	p := Profile{
		Name:             "workstation",
		Limits:           Limits{MaxIterations: &iterations, CompactAt: &compactAt},
		AdditionalPrompt: "You are a terse C bug hunter.",
	}

	var c Config
	ApplyProfile(&c, p)

	if c.MaxIterations == nil || *c.MaxIterations != 12 {
		t.Errorf("max iterations = %v, want the profile's 12", c.MaxIterations)
	}
	if c.CompactAt == nil || *c.CompactAt != 150_000 {
		t.Errorf("compact at = %v, want the profile's 150000", c.CompactAt)
	}
	if c.Additional != "You are a terse C bug hunter." {
		t.Errorf("additional = %q, want the profile's persona", c.Additional)
	}
}

// setupProfileEnvWith writes one profile and one ~/.bulle.yml into a home of the
// test's own, and clears the variables a developer's shell may carry: the suite
// must answer the same on a machine that exports BULLE_PROFILE or
// BULLE_MAX_ITERATIONS as it does in CI.
func setupProfileEnvWith(t *testing.T, profileFile, profileYml, fileYml string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearEnv(t, "BULLE_BACKEND", "NACELLE_BACKEND", "BULLE_PROFILE", "NACELLE_PROFILE",
		"BULLE_MAX_ITERATIONS", "NACELLE_MAX_ITERATIONS")

	pDir := filepath.Join(home, ".bulle", "profiles")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, profileFile), []byte(profileYml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ConfigFile), []byte(fileYml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// clearEnv makes a variable read as unmentioned rather than as empty text.
func clearEnv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		t.Setenv(name, "")
	}
}

func setupProfileEnv(t *testing.T) {
	t.Helper()
	setupProfileEnvWith(t, "fast.yml",
		"name: fast\nprovider:\n  backend: google\n  model: gemini-2.5-flash\n",
		"provider:\n  backend: anthropic\n")
}

func TestFlagProfileOverridesFileBackend(t *testing.T) {
	setupProfileEnv(t)
	cfg, err := Settings("", Config{Profile: "fast"})
	if err != nil {
		t.Fatalf("Settings with flag profile failed: %v", err)
	}
	if cfg.Backend != "google" {
		t.Errorf("flags profile backend = %q, want google", cfg.Backend)
	}
	if cfg.Model != "gemini-2.5-flash" {
		t.Errorf("flags profile model = %q, want gemini-2.5-flash", cfg.Model)
	}
}

func TestEnvProfileOverridesFileBackend(t *testing.T) {
	setupProfileEnv(t)
	t.Setenv("BULLE_PROFILE", "fast")
	cfg, err := Settings("", Config{})
	if err != nil {
		t.Fatalf("Settings with env profile failed: %v", err)
	}
	if cfg.Backend != "google" {
		t.Errorf("env profile backend = %q, want google", cfg.Backend)
	}
	if cfg.Model != "gemini-2.5-flash" {
		t.Errorf("env profile model = %q, want gemini-2.5-flash", cfg.Model)
	}
}
