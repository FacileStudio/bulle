package agent

import (
	"fmt"
	"path/filepath"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/anthropic"

	"github.com/FacileStudio/bulle/internal/settings"
)

// banner is the two lines the transcript opens with.
//
// It opens by naming itself and its own version, which is the one thing on
// screen that a bug report needs and nothing else supplies. Asking somebody
// which build they are on means asking them to quit and run -version, and the
// answer to "it did this yesterday and not today" is usually the number they
// would have had to go and look up. It costs a word on a line that is already
// there.
//
// The backend and model come next, because the failure that costs the most
// is discovering, after composing a question, that the client was pointed
// at a provider you have no key for. Root, skill count and how many
// CLAUDE.md/AGENTS.md files loaded come second — none of it is decorative:
// each is a real "is that actually on" question this client had no way to
// answer before without a debug build.
//
// The one web setting is named in the direction that would otherwise surprise:
// fetch when it is off, because it is on by default and a model that cannot
// read a page it just found needs the reason on screen.
//
// Root is resolved to an absolute path rather than echoed as typed, because
// "-root ." reads the same from any directory bulle happens to be
// launched from and answers nothing on its own.
//
// Whether bash is on is named last and named after the flag, because the
// symptom of it being off arrives from the model rather than from this
// client: asked to build something, it answers that it has no terminal and
// cannot run a command. That is true and deliberate — run_command is
// unconfined, so it stays a decision — but nothing on screen connected it to
// a line in ~/.bulle.yml written once and forgotten. Saying "bash off"
// where the model's own capabilities are listed is the shortest path from
// that answer back to the switch that causes it.
//
// MCP earns its words only when a server was configured: a launch with none is
// the ordinary launch, nothing is offered, and there is no symptom to explain.
// When there is one, both counts are named
// rather than only the servers — a server that starts and offers nothing is
// the one failure this client cannot otherwise show, since it is an error
// nowhere and its only trace is a model that never reaches for the tool.
func banner(backend nacelle.Backend, config settings.Config, found loaded, mcp connected, version string) string {
	model := resolvedModel(backend, config)
	root := absolute(config.Root)
	bash := "bash off"
	if *config.Bash {
		bash = "bash on"
	}
	line := fmt.Sprintf("bulle %s · %s · %s\n%s · %s · %s · %s", version, backend.Name(), model, root,
		countedNoun(len(found.skills), "skill"), countedNoun(found.contextFiles, "context file"), bash)

	if mcp.servers > 0 {
		line += " · " + countedNoun(mcp.servers, "MCP server") + ", " + countedNoun(mcp.tools, "tool")
	}
	if !*config.Fetch {
		line += " · fetch off"
	}
	return line
}

// resolvedModel names the model this session runs on: the configured name,
// else the backend's own default, else anthropic's. An empty model in the
// config is the ordinary case — the provider fills its own default — and the
// banner and the status line both have to name the one that actually answers,
// so neither reads the raw setting.
func resolvedModel(backend nacelle.Backend, config settings.Config) string {
	if config.Model != "" {
		return config.Model
	}
	if withModel, ok := backend.(interface{ Model() string }); ok && withModel.Model() != "" {
		return withModel.Model()
	}
	return anthropic.DefaultModel
}

// countedNoun is "N noun" or "N nouns" — the one piece of English this
// client bothers to pluralize, because the banner reads it on every launch.
func countedNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// absolute is the root as a path that means the same thing from anywhere.
// "-root ." reads identically from every directory bulle was launched in
// and so answers nothing on its own — neither for the person reading the
// banner nor for the model reading the system prompt, which are the two
// callers.
//
// A target-labeled root ("target:<name>", set by remote and sandbox
// sessions with no configured workdir) is echoed back as-is: it names the
// machine on the other end of the SSH connection, not a directory on this
// one, and resolving it would only rename the directory bulle was launched
// from.
func absolute(root string) string {
	if settings.IsTargetRoot(root) {
		return root
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}
