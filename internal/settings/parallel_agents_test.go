package settings

import (
	"testing"
)

// On by default: delegation is enabled out of the box.
func TestParallelAgentsDefaultOn(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.ParallelAgents {
		t.Error("parallel_agents default off")
	}
}

// The whole precedence chain has to carry the toggle, or the layer that
// turns it on is not the layer that decides.
func TestParallelAgentsFollowThePrecedenceChain(t *testing.T) {
	written(t, "tools:\n  parallel_agents: false")
	t.Setenv("BULLE_PARALLEL_AGENTS", "true")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.ParallelAgents {
		t.Error("the environment did not beat the file")
	}
}

// The settings layer can only resolve the yaml into the config; mounting the
// tool is agent wiring, asserted in the agent package's delegate_test.go. What
// the layer owes that mount is that parallel_agents: true reaches the resolved
// config.
func TestParallelAgentsTrueInTheFileTurnsTheMountOn(t *testing.T) {
	written(t, "tools:\n  parallel_agents: true")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.ParallelAgents {
		t.Error("parallel_agents: true in the file left the mount off")
	}
}

func TestMaxConcurrencyDefaultsTo16(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.MaxConcurrency == nil || *config.MaxConcurrency != 16 {
		t.Errorf("max_concurrency got %v, want 16", config.MaxConcurrency)
	}
}

func TestMaxConcurrencyFollowsPrecedenceChain(t *testing.T) {
	written(t, "limits:\n  max_concurrency: 8")
	t.Setenv("BULLE_MAX_CONCURRENCY", "32")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.MaxConcurrency == nil || *config.MaxConcurrency != 32 {
		t.Errorf("max_concurrency got %v, want 32", config.MaxConcurrency)
	}
}

func TestMaxParallelAgentsFromEnvAndFile(t *testing.T) {
	written(t, "limits:\n  max_parallel_agents: 4")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.MaxParallelAgents == nil || *config.MaxParallelAgents != 4 {
		t.Errorf("max_parallel_agents got %v, want 4", config.MaxParallelAgents)
	}

	t.Setenv("BULLE_MAX_PARALLEL_AGENTS", "24")
	config, err = settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if config.MaxParallelAgents == nil || *config.MaxParallelAgents != 24 {
		t.Errorf("max_parallel_agents got %v, want 24", config.MaxParallelAgents)
	}
}
