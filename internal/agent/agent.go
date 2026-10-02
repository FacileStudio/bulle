// Package agent provides the agent configuration, tools, and execution loop.
package agent

import (
	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/diagnostics"
	"github.com/FacileStudio/bulle/internal/provider"
	"github.com/FacileStudio/bulle/internal/settings"
)

// Assembling the agent from settings, split out of main.go because that file
// sat on filet's 250-line cap and this is the seam already there: main.go
// decides what the settings are, and this turns them into the thing that
// answers. Nothing here reads a flag or touches the terminal.

// built is what assemble builds: the agent, the backend it answers on, and the
// nacelle.Config it was built from. Grouped so assemble returns two values
// instead of four — the config is only there so /parallel can clone it for its
// own detached fan-out; callers that do not delegate ignore it.
type built struct {
	agent   *nacelle.Agent
	backend nacelle.Backend
	config  nacelle.Config
}

// build assembles the agent the settings describe: backendFor and assemble in
// one call, for the callers that want an agent and nothing else. The
// interactive session resolves the backend itself first, because the IDE
// surface is opened before the agent exists and has to name the model an empty
// provider.model leaves to the provider.
func build(config *settings.Config, local []nacelle.Tool, approve nacelle.Approve, hooks map[nacelle.HookPoint][]nacelle.Hook) (built, error) {
	backend, err := backendFor(config)
	if err != nil {
		return built{}, err
	}
	return assemble(*config, backend, local, approve, hooks)
}

// backendFor resolves the keys a config left to a command and builds the
// backend the session answers on.
//
// config is taken by pointer because a key a file left to a command is resolved
// once, here, and the caller needs the result: every path that builds an agent
// arrives through this function, so this is the one place an api_key_command
// can run — and it runs after every layer has merged, so a job's own command
// wins over the session's the same way its literal key does.
func backendFor(config *settings.Config) (nacelle.Backend, error) {
	if err := settings.ResolveKeys(config); err != nil {
		return nil, err
	}
	return chosen(*config)
}

// assemble builds the agent around a backend that is already chosen, and hands
// that backend back so the caller can say which one answered. approve is nil
// unless -approve-tools was asked for — see nacelle.Approve's own doc comment
// for why nil, not a rubber-stamp function, is what "off" means here.
//
// The three reasoning settings fold into one nacelle.Thinking here, and the
// one rename in that fold is worth knowing about: this client's -thinking
// becomes Show, which decides what the transcript displays and nothing else:
// the model reasons, is billed, and replays its reasoning either way.
//
// built.config is the nacelle.Config the agent was built from, kept so a
// /parallel fan-out runs its own agents from the same tools, system prompt and
// iteration ceiling instead of a hand-built subset.
func assemble(config settings.Config, backend nacelle.Backend, local []nacelle.Tool, approve nacelle.Approve, hooks map[nacelle.HookPoint][]nacelle.Hook) (built, error) {
	approve = unwrapCallTool(approve)

	retrying := nacelle.Retry(backend, nacelle.RetryOptions{})
	local, err := withParallelAgents(config, retrying, local, approve)
	if err != nil {
		return built{}, err
	}
	local = withTasks(config, local)
	local, hooks = withDiagnostics(config, local, hooks)

	cfg := nacelle.Config{
		Backend: retrying,
		System:  config.System,
		Thinking: nacelle.Thinking{
			Effort: nacelle.Effort(config.Effort),
			Budget: *config.Budget,
			Show:   *config.Thinking,
		},
		Tools:         local,
		MaxIterations: *config.MaxIterations,
		Approve:       approve,
		Hooks:         hooks,
	}
	agent, err := nacelle.New(cfg)
	if err != nil {
		return built{}, err
	}
	return built{agent: agent, backend: backend, config: cfg}, nil
}

// withDiagnostics adds the diagnostics tool and installs the post-edit gate
// chain when the toggle is on, handing both back untouched when it is off. Split
// out of build to keep that function inside filet's length cap once key
// resolution joined it; the two writes are one decision, so they stay together.
func withDiagnostics(config settings.Config, local []nacelle.Tool, hooks map[nacelle.HookPoint][]nacelle.Hook) ([]nacelle.Tool, map[nacelle.HookPoint][]nacelle.Hook) {
	if !settings.DerefBool(config.Diagnostics) {
		return local, hooks
	}
	diagnostics.UseChain(chainOf(config.Gates))
	return append(local, diagnostics.Tool()), withDiagnosticsHook(hooks)
}

// chosen builds the backend the settings ask for.
//
// An unknown name is refused rather than quietly falling back to a model the
// caller did not choose and will be billed for, which is the same reason the
// library itself ships no default backend.
func chosen(config Config) (nacelle.Backend, error) {
	return provider.New(provider.Config{
		Backend: config.Backend,
		Model:   config.Model,
		BaseURL: config.BaseURL,
		APIKey:  config.APIKey,
	})
}
