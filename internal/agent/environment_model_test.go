package agent

import (
	"strings"
	"testing"
)

func TestEnvironmentNamesTheSessionModel(t *testing.T) {
	var config Config
	config.Model, config.Backend = "anthropic/claude-haiku-5.5", "openrouter"
	got := modelNote(config)
	if !strings.Contains(got, "anthropic/claude-haiku-5.5") || !strings.Contains(got, "openrouter") {
		t.Errorf("note = %q", got)
	}
	if modelNote(Config{}) != "" {
		t.Error("no model means no note")
	}
}
