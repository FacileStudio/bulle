package settings

import (
	"slices"
	"testing"
)

func TestMCPFilesFromYAML(t *testing.T) {
	written(t, "sources:\n  mcp_files:\n    - /path/to/claude.json\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !slices.Contains(config.MCPFiles, "/path/to/claude.json") {
		t.Errorf("config.MCPFiles = %v, want /path/to/claude.json", config.MCPFiles)
	}
}

func TestMCPFilesFromEnvironment(t *testing.T) {
	written(t, "")
	t.Setenv("BULLE_MCP_FILES", "/env/one.json:/env/two.json")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if len(config.MCPFiles) != 2 || config.MCPFiles[0] != "/env/one.json" || config.MCPFiles[1] != "/env/two.json" {
		t.Errorf("config.MCPFiles = %v, want [/env/one.json /env/two.json]", config.MCPFiles)
	}
}
