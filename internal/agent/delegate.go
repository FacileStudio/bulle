package agent

import (
	"context"
	"encoding/json"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/tui"
)

// withParallelAgents mounts the delegation tools when the settings ask for them.
// It exists so build stays a readable sequence of wiring rather than growing a
// branch per optional tool: the delegates share the parent's wrapped backend,
// system prompt, tools and iteration ceiling, and report their spend to the
// session the same way the parent's own turns do.
func withParallelAgents(config settings.Config, backend nacelle.Backend, local []nacelle.Tool, approve nacelle.Approve) ([]nacelle.Tool, error) {
	if !*config.ParallelAgents {
		return local, nil
	}
	concurrency := resolveConcurrency(config.MaxConcurrency, config.MaxParallelAgents)
	parallel, err := nacelle.NewParallelSubAgentTool(nacelle.Config{
		Backend:       backend,
		System:        config.System,
		Tools:         local,
		MaxIterations: *config.MaxIterations,
	}, nacelle.ParallelSubAgentOptions{
		Description: "Delegate independent sub-tasks to parallel assistant runs. " +
			"Each task runs in its own agent concurrently; dispatch them, then end your turn and " +
			"return to ready — the person's next input covers any concurrent work, and the harness " +
			"hands you the completed results to synthesize when they finish. Do not keep planning or " +
			"issuing further tool calls after the fan-out has started. Provide a short 4-7 word title " +
			"describing each session for the status line alongside the task instructions.",
		Approve:        delegateApprovals(approve),
		Usage:          tui.DelegateUsage,
		Detach:         true,
		Results:        tui.PostDetached,
		Tool:           tui.ReportSubagentTool,
		ToolDone:       tui.ReportSubagentDone,
		LiveUsage:      tui.ReportSubagentUsage,
		MaxConcurrency: concurrency,
	})
	if err != nil {
		return nil, err
	}

	return withParallelCancelTool(append(local, returnControl{parallel}))
}

// returnControl wraps the parallel tool so the stub the model reads — a bare
// {"started":N,"batch":...} from the SDK — carries the return-control rule
// with it. The tool description states the contract, but the stub arrives at
// the exact moment a model is tempted to keep the main thread open, and a
// result weighs more on that decision than a description does.
type returnControl struct {
	nacelle.Tool
}

func (r returnControl) Schema() map[string]any {
	itemProps := map[string]any{
		"title": map[string]any{
			"type":        "string",
			"description": "A short, 4-7 word descriptive title for this session shown in the status line (e.g. 'audit auth middleware')",
		},
		"task": map[string]any{
			"type":        "string",
			"description": "The detailed instructions and prompt for the subagent run",
		},
	}
	items := map[string]any{
		"type":       "object",
		"properties": itemProps,
		"required":   []string{"title", "task"},
	}
	taskProp := map[string]any{
		"type":        "array",
		"description": "List of independent subagent tasks to run in parallel",
		"minItems":    1,
		"items":       items,
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"tasks": taskProp},
		"required":   []string{"tasks"},
	}
}

func (r returnControl) Run(ctx context.Context, input json.RawMessage) (string, error) {
	out, err := r.Tool.Run(ctx, normalizeParallelInput(input))
	if err == nil {
		out += "\nFan-out dispatched. End your turn now: the parallel agents run detached, " +
			"their results stream back to the harness, and you are re-engaged to synthesize " +
			"them when they finish. Do not keep calling tools on the main thread."
	}
	return out, err
}

func normalizeParallelInput(input json.RawMessage) json.RawMessage {
	tasks, _ := tui.ParseParallelTasks(string(input))
	if len(tasks) == 0 {
		return input
	}
	out, err := json.Marshal(struct {
		Tasks []string `json:"tasks"`
	}{Tasks: tasks})
	if err != nil {
		return input
	}
	return out
}

func withParallelCancelTool(local []nacelle.Tool) ([]nacelle.Tool, error) {
	cancel, err := nacelle.NewParallelCancelTool()
	if err != nil {
		return nil, err
	}
	return append(local, cancel), nil
}

// delegateApprovals is the policy the nested run answers to. It has to be
// stated, because the SDK's default for a nil ParallelSubAgentOptions.Approve is
// deny-all: leaving it unset hands the delegate the parent's whole tool set
// and then refuses every call it makes, which is a tool whose description
// promises wide searches and log dumps and which can do neither.
func delegateApprovals(approve nacelle.Approve) nacelle.Approve {
	if approve != nil {
		return approve
	}
	return func(context.Context, string, json.RawMessage) bool { return true }
}

func resolveConcurrency(concurrency, parallelAgents *int) int {
	if concurrency != nil && *concurrency > 0 && *concurrency != 16 {
		return *concurrency
	}
	if parallelAgents != nil && *parallelAgents > 0 {
		return *parallelAgents
	}
	if concurrency != nil && *concurrency > 0 {
		return *concurrency
	}
	return 16
}
