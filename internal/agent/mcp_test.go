package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/mcp/client"
)

func TestNoMCPConfigStartsNothingAndChangesNothing(t *testing.T) {
	local := []nacelle.Tool{}

	mcp, grown, err := mcpTools(defaults(), local)
	if err != nil {
		t.Fatalf("mcpTools: %v", err)
	}
	defer func() { _ = mcp.set.Close() }()

	if mcp.servers != 0 || mcp.tools != 0 {
		t.Errorf("connected = %+v, want nothing connected", mcp)
	}
	if len(grown) != 0 {
		t.Errorf("tools = %+v, want the local set handed straight back", grown)
	}
	if mcp.set == nil {
		t.Error("set = nil, want an empty Set the caller can close unconditionally")
	}
}

func TestTheBannerSaysNothingAboutMCPWhenNoneIsConfigured(t *testing.T) {
	got := testBanner(&answeringStub{}, asSettled(Config{Session: Session{Root: "."}}), loaded{}, connected{})
	if strings.Contains(got, "MCP") {
		t.Errorf("banner = %q, want no mention of MCP when none is configured", got)
	}
}

func TestAServerThatWillNotStartEndsTheRun(t *testing.T) {
	config := defaults()
	config.MCP = map[string]client.ServerDef{
		"ledger": {Command: "/nonexistent/bulle-mcp"},
	}

	_, _, err := mcpTools(config, nil)
	if err == nil {
		t.Fatal("a server that cannot start was accepted")
	}
	if !strings.Contains(err.Error(), "ledger") {
		t.Errorf("error = %v, want it to name the server that would not start", err)
	}
}

func TestAnUnreadableMCPFileEndsTheRun(t *testing.T) {
	config := defaults()
	config.MCPFiles = []string{filepath.Join(t.TempDir(), "never-written.json")}

	if _, _, err := mcpTools(config, nil); err == nil {
		t.Fatal("a config file that was never there was accepted")
	}
}

func TestMCPFromBulleYmlIsInlineAndTheFlagNamesFiles(t *testing.T) {
	written(t, "sources:\n  mcp:\n    mycelium:\n      command: mycelium\n      args: [mcp]\n")

	config, err := resolveSettings(Config{Sources: Sources{MCPFiles: []string{"/from/the/flag.json"}}})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if got := config.MCP["mycelium"]; got.Command != "mycelium" || len(got.Args) != 1 {
		t.Errorf("MCP[mycelium] = %+v, want the inline server from the file", got)
	}
	if want := []string{"/from/the/flag.json"}; !slices.Equal(config.MCPFiles, want) {
		t.Errorf("MCPFiles = %v, want %v", config.MCPFiles, want)
	}
}

func TestNoMCPKeyAnywhereLeavesMCPUnset(t *testing.T) {
	written(t, "provider:\n  backend: openrouter\n")

	config, err := resolveSettings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if len(config.MCP) != 0 {
		t.Errorf("mcp = %v, want nothing configured", config.MCP)
	}
	if len(config.MCPFiles) != 0 {
		t.Errorf("mcp_files = %v, want nothing configured", config.MCPFiles)
	}
}

func TestUnwrapCallToolAsksByTheBridgedName(t *testing.T) {
	var seen []string
	gate := unwrapCallTool(func(_ context.Context, name string, _ json.RawMessage) bool {
		seen = append(seen, name)
		return true
	})
	if !gate(context.Background(), "call_tool", []byte(`{"name":"srv_echo","arguments":{"hi":true}}`)) {
		t.Fatal("call_tool was refused")
	}
	if len(seen) != 1 || seen[0] != "srv_echo" {
		t.Fatalf("asked about %v, want only srv_echo", seen)
	}
	if !gate(context.Background(), "read_file", nil) || len(seen) != 2 || seen[1] != "read_file" {
		t.Fatalf("a plain tool did not pass through: %v", seen)
	}
	if !gate(context.Background(), "call_tool", []byte(`{"arguments":{}}`)) {
		t.Fatal("call_tool without a name was refused")
	}
	if len(seen) != 3 || seen[2] != "call_tool" {
		t.Fatalf("malformed input did not fall back to call_tool: %v", seen)
	}
}

func TestUnwrapCallToolLeavesApprovalOffAlone(t *testing.T) {
	if unwrapCallTool(nil) != nil {
		t.Fatal("nil approve came back non-nil")
	}
}
