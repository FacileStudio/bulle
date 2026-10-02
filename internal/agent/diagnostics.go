package agent

import (
	"context"
	"encoding/json"
	"maps"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/diagnostics"
	"github.com/FacileStudio/bulle/internal/settings"
)

// The diagnostics loop: filet findings injected after every edit, plus the
// pull tool. Built in, not a settings hook, so the kill switch is the single
// tools.diagnostics toggle and the injection lands even with no hooks file.
// When the settings configure gates the agent builds one chain from them and
// both the hook and the pull tool consult it; no gates keeps the filet-only
// path.

// editTools are the tools whose result a diagnostics injection rides on.
var editTools = map[string]bool{"edit_file": true, "write_file": true}

// editInput is the slice of the model's raw tool arguments the injection
// needs. The input arrives undecoded, and malformed JSON is silence: a hook
// error would deny the edit it is reporting on.
type editInput struct {
	Path string `json:"path"`
}

// gateChain is the thin surface the agent needs from the diagnostics gate
// chain: injection for the edit hook and the full run for the pull tool. The
// chain itself is being built in
// internal/diagnostics (gate.go and chain.go); until its constructor lands
// there this is the contract the seam below fills, and a nil chain keeps every
// caller on the filet-only path.
type gateChain interface {
	InjectFile(ctx context.Context, path string) string
	Run(ctx context.Context, path string, repo bool) (string, error)
}

// chainOf builds the gate chain the settings describe, or nil when no gates
// are configured. It is a seam like runFilet rather than a factory: the
// diagnostics package's own chain constructor takes over the body when
// gate.go and chain.go land, and until then configured gates stay silent
// rather than half-wired.
var chainOf = func(specs []settings.GateSpec) gateChain {
	gates := make([]diagnostics.Gate, 0, len(specs))
	for _, s := range specs {
		gates = append(gates, diagnostics.Gate{
			Name:        s.Name,
			Cmd:         s.Command,
			TimeoutSecs: s.TimeoutSecs,
			Format:      s.Format,
		})
	}
	if len(gates) == 0 {
		return nil
	}
	return diagnostics.NewChain(gates)
}

// withDiagnosticsHook returns the hooks map with the post-edit injection
// appended, copying on write so the caller's map is untouched.
func withDiagnosticsHook(hooks map[nacelle.HookPoint][]nacelle.Hook) map[nacelle.HookPoint][]nacelle.Hook {
	diag := func(ctx context.Context, ev nacelle.HookEvent) nacelle.HookResult {
		if !editTools[ev.Tool] || ev.Err != nil {
			return nacelle.HookResult{}
		}
		var in editInput
		if err := json.Unmarshal([]byte(ev.Input), &in); err != nil || in.Path == "" {
			return nacelle.HookResult{}
		}
		return nacelle.HookResult{Inject: diagnostics.InjectFile(ctx, in.Path)}
	}
	out := maps.Clone(hooks)
	if out == nil {
		out = map[nacelle.HookPoint][]nacelle.Hook{}
	}
	out[nacelle.AfterToolCall] = append(out[nacelle.AfterToolCall], diag)
	return out
}
