package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/FacileStudio/nacelle"
)

func TestParseTargetModel(t *testing.T) {
	backend, model := parseTargetModel("openai/gpt-5.4", "anthropic")
	if backend != "openai" || model != "gpt-5.4" {
		t.Errorf("got %s/%s, want openai/gpt-5.4", backend, model)
	}

	backend, model = parseTargetModel("claude-opus-5", "anthropic")
	if backend != "anthropic" || model != "claude-opus-5" {
		t.Errorf("got %s/%s, want anthropic/claude-opus-5", backend, model)
	}
}

func TestModelCommandRefusesWhenBusy(t *testing.T) {
	m := &Model{}
	m.run.busy = true

	cmd := m.modelCmd("gpt-4o")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if len(m.unprinted) == 0 || !strings.Contains(m.unprinted[len(m.unprinted)-1], "cannot switch model while a run is in progress") {
		t.Errorf("expected busy message, got %+v", m.unprinted)
	}
}

func TestModelCommandOpensMenuWhenEmpty(t *testing.T) {
	m := sized()
	m.activeBackend = "anthropic"
	m.activeModel = "claude-opus-5"
	m.delegate = nacelle.Config{System: "test"}
	if cmd := m.modelCmd(""); cmd != nil {
		t.Errorf("expected nil cmd from open menu, got %v", cmd)
	}
	if !m.menu.Open() || len(m.menu.Items) == 0 || !m.modelPicker {
		t.Fatal("expected model menu to be open with items and picker active")
	}
	if !m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyEnter}) {
		t.Fatal("expected enter to be handled by model menu")
	}
	if m.menu.Open() || m.modelPicker {
		t.Fatal("expected menu to be dismissed and picker deactivated")
	}
}

func TestModelCommandSwitchDirectModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	m := &Model{}
	m.activeBackend = "anthropic"
	m.activeModel = "claude-opus-5"
	m.delegate = nacelle.Config{System: "test"}

	if cmd := m.modelCmd("openai/gpt-4o"); cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	if m.activeBackend != "openai" || m.activeModel != "gpt-4o" {
		t.Errorf("got %s/%s, want openai/gpt-4o (unprinted: %v)", m.activeBackend, m.activeModel, m.unprinted)
	}
}

func TestModelCommandSwitchProfile(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	profilesDir := filepath.Join(dir, ".bulle", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profileYml := `name: fast
provider:
  backend: google
  model: gemini-2.5-flash
reasoning:
  effort: low
`
	if err := os.WriteFile(filepath.Join(profilesDir, "fast.yml"), []byte(profileYml), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Model{}
	m.activeBackend = "anthropic"
	m.activeModel = "claude-opus-5"
	m.delegate = nacelle.Config{System: "test"}

	if cmd := m.modelCmd("fast"); cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	if m.activeBackend != "google" || m.activeModel != "gemini-2.5-flash" {
		t.Errorf("got %s/%s, want google/gemini-2.5-flash", m.activeBackend, m.activeModel)
	}
}

func TestStatusCmdOutputsActiveModel(t *testing.T) {
	m := &Model{}
	m.activeBackend = "anthropic"
	m.activeModel = "claude-opus-5"
	m.statusCmd()
	if len(m.unprinted) == 0 {
		t.Fatal("expected status output, got none")
	}
	last := m.unprinted[len(m.unprinted)-1]
	if !strings.Contains(last, "model · anthropic/claude-opus-5") {
		t.Fatalf("expected model output, got %q", last)
	}
}

func TestApplyProfileSwitchResetsReasoningBudget(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	profilesDir := filepath.Join(dir, ".bulle", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profileYml := `name: nobudget
provider:
  backend: google
  model: gemini-2.5-flash
reasoning:
  effort: medium
`
	if err := os.WriteFile(filepath.Join(profilesDir, "nobudget.yml"), []byte(profileYml), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Model{}
	m.activeBackend = "anthropic"
	m.activeModel = "claude-opus-5"
	m.delegate = nacelle.Config{
		System:   "test",
		Thinking: nacelle.Thinking{Budget: 4096, Effort: nacelle.EffortHigh},
	}

	if cmd := m.modelCmd("nobudget"); cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	if m.delegate.Thinking.Budget != 0 {
		t.Errorf("expected budget to be reset to 0, got %d", m.delegate.Thinking.Budget)
	}
	if m.delegate.Thinking.Effort != nacelle.Effort("medium") {
		t.Errorf("expected effort to be medium, got %s", m.delegate.Thinking.Effort)
	}
}

func TestDirectModelSwitchPreservesEndpoint(t *testing.T) {
	m := &Model{}
	m.activeBackend = "openai"
	m.activeModel = "gpt-4o"
	m.activeBaseURL = "http://localhost:8000/v1"
	m.activeAPIKey = "custom-key"
	m.delegate = nacelle.Config{System: "test"}

	if cmd := m.modelCmd("openai/gpt-5.4"); cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	if m.activeBaseURL != "http://localhost:8000/v1" {
		t.Errorf("expected baseURL preserved, got %q", m.activeBaseURL)
	}
	if m.activeAPIKey != "custom-key" {
		t.Errorf("expected apiKey preserved, got %q", m.activeAPIKey)
	}
}
