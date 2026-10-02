package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/mcp/client"

	"github.com/FacileStudio/bulle/internal/settings"
)

// connected is the live session to every MCP server this launch was told
// about, and the two counts the banner reports back.
//
// The counts are carried rather than derived because only one of them can be:
// a Set knows how many tools it bridged and not how many servers produced
// them, and "2 MCP servers, 0 tools" is exactly the launch somebody needs to
// see spelled out. Holding both also lets the banner be tested without a
// subprocess anywhere near it.
//
// names are the enabled servers, and catalog says whether the bridged set was
// mounted whole or as the three catalog tools — both so the prompt's MCP
// note can say what the model is actually holding.
type connected struct {
	set     *client.Set
	servers int
	tools   int
	names   []string
	catalog bool
}

// mcpTools starts every MCP server the settings name and hands local back with
// their tools appended. The caller owns the session and must close it — each
// server is a process nothing else will reap.
//
// The local tool set goes in and comes out grown, rather than the bridged half
// coming back on its own for the caller to append. It is the same reason
// declareWeb is a function of its own: run() sits at the statement budget the
// gate allows, and an append on a line there costs one that this costs nothing.
//
// Naming none is a working no-op and is meant to stay one. Load over no paths
// finds no servers, Connect over no servers starts nothing and still hands
// back a Set that closes cleanly, so the great majority of people — who have
// never written one of these files — get the launch they had before this
// existed, down to the banner.
//
// A server that will not start ends the run, which is the opposite of how
// skills and project context fail. Those are discovered, so finding nothing is
// indistinguishable from there being nothing to find; this was asked for by
// name, and a tool that is quietly missing reads as a model refusing to work
// rather than as a server that is down. client.Connect refuses to degrade to
// the servers that did come up for the same reason, one level further in.
//
// Both errors go back unwrapped: each already begins with nacelle/mcp/client
// and names the file or the server it is about, so a "connecting to the MCP
// servers" in front of it says nothing the reader has not just read.
//
// context.Background is deliberate rather than lazy. Connect bounds its own
// handshake per server, and the sessions have to outlive this call by the
// entire length of the run — a context cancelled on the way out of here would
// take every server down with it before the model was asked anything.
//
// Nothing here goes looking for a .mcp.json under root, and that absence is
// the decision rather than an oversight. Such a file names executables to run,
// so honouring one found on the way past would start a stranger's process on
// the strength of having cd'd into their repository. That is strictly worse
// than the project-local skills this client already gates behind
// ~/.bulle/trust.json and -trust-skills: a skill is text the model may
// decline to act on, and this is a subprocess started before the model is
// asked anything at all. Doing it safely means that same trust gate, which is
// a feature of its own rather than a line in this function.
func mcpTools(config settings.Config, local []nacelle.Tool) (connected, []nacelle.Tool, error) {
	ensureUserPath()
	defs := map[string]client.ServerDef{}
	if config.MCP != nil {
		for name, def := range config.MCP {
			defs[name] = expandServerDef(def)
		}
	}
	flagDefs, err := client.LoadDefs(expandPaths(config.MCPFiles)...)
	if err != nil {
		return connected{}, nil, err
	}
	for name, def := range flagDefs {
		defs[name] = expandServerDef(def)
	}

	servers, err := client.Parse(defs)
	if err != nil {
		return connected{}, nil, err
	}
	if settings.DerefBool(config.EnvIsolation) {
		servers = client.WithEnvIsolation(servers, true)
	}

	set, err := client.Connect(context.Background(), servers...)
	if err != nil {
		return connected{}, nil, err
	}
	bridged := set.Tools()
	return connected{set: set, servers: len(servers), tools: len(bridged), names: enabledNames(defs), catalog: len(bridged) > mcpCatalogThreshold},
		grown(local, bridged), nil
}

// unwrapCallTool makes the approval gate see the bridged tool a call_tool
// invocation names, rather than call_tool itself. In catalog mode every
// bridged tool runs through that one tool, so asked as-is the gate would show
// "call_tool" for all of them and one Allow-for-session would open every
// bridged tool at once. When the input does not carry a legible name the
// original call stands, malformed input being the refusal the gate exists to
// catch. Applied to nil unchanged, so approval-off stays approval-off.
func unwrapCallTool(approve nacelle.Approve) nacelle.Approve {
	if approve == nil {
		return nil
	}
	return func(ctx context.Context, name string, input json.RawMessage) bool {
		if name == "call_tool" {
			var in struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(input, &in); err == nil && in.Name != "" {
				return approve(ctx, in.Name, normalizeCallToolArgs(in.Arguments))
			}
		}
		return approve(ctx, name, input)
	}
}

// enabledNames keeps the server names Parse will act on — the map's keys
// minus the disabled ones — sorted, because a map iterates in no order and
// the note naming them is prompt text whose order should not shuffle
// between runs.
func enabledNames(defs map[string]client.ServerDef) []string {
	var names []string
	for name, def := range defs {
		if !def.Disabled {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// mcpNote tells the model what the MCP-bridged half of its tool set is and
// which layer it belongs to — the capability self-knowledge the built-in
// descriptions cannot carry, since a bridged tool's description is the
// server's own text. Every bridged name is prefixed with its server's name
// by the bridge itself, so grouping by server is mechanical from the tool
// list.
//
// With nothing mounted the note is empty rather than a claim of zero: the
// absence of MCP is the ordinary shape of most launches and costs no tokens.
func mcpNote(mcp connected) string {
	if mcp.servers == 0 {
		return ""
	}
	var body strings.Builder
	fmt.Fprintf(&body, "\nMCP-bridged tools are mounted from these servers: %s. Their names are prefixed with their server's name; group them by that prefix when several serve one job. Prefer using an MCP-bridged tool over shell commands in run_command whenever one fits the task.\n", strings.Join(mcp.names, ", "))
	if mcp.catalog {
		body.WriteString("\nThe set is too large to carry whole, so search_tools, get_tool_details and call_tool stand in for it: search_tools before assuming a bridged tool exists, get_tool_details before the first call to a tool whose schema you have not read, call_tool to execute. The three are the harness's own; every other name they surface is a server's.\n")
	}
	return body.String()
}

// grown appends the bridged half to the local set — or, when the bridged set
// is too big to mount whole, the three catalog tools that stand in for it.
// One set of definitions resident per call either way, and never re-sorted:
// order is stable across a conversation, which is what keeps the prompt cache
// alive.
func grown(local, bridged []nacelle.Tool) []nacelle.Tool {
	if len(bridged) > mcpCatalogThreshold {
		return append(local, newCatalogTools(bridged)...)
	}
	return append(local, bridged...)
}
