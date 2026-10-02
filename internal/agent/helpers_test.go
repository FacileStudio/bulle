package agent

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

func asSettled(c Config) Config {
	if c.Bash == nil {
		off := false
		c.Bash = &off
	}
	if c.Fetch == nil {
		on := true
		c.Fetch = &on
	}
	return c
}

type answeringStub struct{ received nacelle.Request }

func (s *answeringStub) Name() string                       { return "stub" }
func (s *answeringStub) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{} }
func (s *answeringStub) Stream(_ context.Context, request nacelle.Request) iter.Seq2[nacelle.Event, error] {
	s.received = request
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindText, Text: "done"}, nil)
	}
}
func (s *answeringStub) CountTokens(_ context.Context, _ nacelle.Request) (int64, error) {
	return 10, nil
}

func written(t *testing.T, body string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(home, settings.ConfigFile), []byte(body), 0o600); err != nil {
		t.Fatalf("writing the config: %v", err)
	}
}

func writeSkill(t *testing.T, dir, frontmatterBody string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := fmt.Sprintf("---\n%s\n---\n# Skill\n", frontmatterBody)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func resolveSettings(flags Config) (Config, error) {
	return settings.Settings(DefaultSystemPrompt(), flags)
}

func testBanner(backend nacelle.Backend, config settings.Config, found loaded, mcp connected) string {
	return banner(backend, config, found, mcp, "dev")
}
