package agent

import (
	"context"
	"testing"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

type dummyInput struct{}

func dummyTool(name string) nacelle.Tool {
	t, _ := nacelle.NewTool(name, "test tool", func(_ context.Context, _ dummyInput) (string, error) {
		return "ok", nil
	})
	return t
}

// It runs on a home of the test's own: setupAgentCustomTools resolves settings
// through the real chain, so without that it read the developer's ~/.bulle.yml
// and whichever profile that names.
func TestSetupAgentCustomTools(t *testing.T) {
	writeHome(t, "")
	flags := settings.Defaults("test")
	custom := []nacelle.Tool{dummyTool("remote_tool")}
	prep, err := setupAgentCustomTools(flags, custom, false)
	if err != nil {
		t.Fatalf("setupAgentCustomTools error: %v", err)
	}
	found := false
	for _, tool := range prep.local {
		if tool.Name() == "remote_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected remote_tool in prepared tools")
	}
}

func TestRunHeadlessWithTools_CleanupRuns(t *testing.T) {
	flags := settings.Defaults("test")
	flags.Provider = settings.Provider{Backend: "unknown"}
	cleaned := false
	cleanup := func() {
		cleaned = true
	}
	custom := []nacelle.Tool{dummyTool("remote_tool")}
	err := RunHeadlessWithTools("hello", flags, custom, cleanup)
	if err == nil {
		t.Fatal("expected error with unknown backend")
	}
	if !cleaned {
		t.Fatal("expected cleanup to be called on exit")
	}
}

func TestRunSessionWithTools_CleanupRuns(t *testing.T) {
	flags := settings.Defaults("test")
	flags.Provider = settings.Provider{Backend: "unknown"}
	cleaned := false
	cleanup := func() {
		cleaned = true
	}
	custom := []nacelle.Tool{dummyTool("remote_tool")}
	err := RunSessionWithTools("0.0.1", flags, custom, cleanup)
	if err == nil {
		t.Fatal("expected error with unknown backend")
	}
	if !cleaned {
		t.Fatal("expected cleanup to be called on exit")
	}
}
