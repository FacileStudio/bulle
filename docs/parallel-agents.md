# Parallel Sub-Agents — Actual Implementation

This is the implementation doc, not the original plan. There are two delegation paths, and both are non-blocking — the main thread stays free whether the fan-out came from a person typing a command or from the model choosing a tool itself.

- The **command** path — `/parallel a, b, c`. The TUI launches the fan-out itself, outside any parent run, and streams results in. `parallel_cmd.go`.
- The **model-callable** path — `parallel_subagent` as a tool the parent model calls mid-turn. nacelle v0.16.0's `Detach` mode makes the tool return a stub immediately and run the fan-out in the background, so the parent's turn keeps going and nothing queues.

Both end up as the same thing: a "started N parallel agents" message, one titled row per subagent under the prompt, and results streamed in as they finish.

## What shipped

- `/parallel task1, task2, task3` — detached user-facing command
- `internal/tui/parallel_cmd.go` — parser + detached fan-out launcher
- `internal/tui/parallel_detached.go` — result watch + per-result handlers
- `internal/agent/delegate.go` — mounts `NewParallelSubAgentTool` in `Detach` mode, wired to `tui.PostDetached` and the model-callable path
- nacelle `DelegateParallel` + `ParallelSubAgentOptions.Detach` — the detached fan-out machinery (subagent_parallel.go)
- `internal/tasks/taskplan.go` — agent-facing task-plan tooling

## How a fan-out works

1. Someone (a person or the model) opens a fan-out with tasks a, b, c. The command path splits on `,`; the model path calls `parallel_subagent`, which returns `{"started":3,"batch":key}` immediately and keeps running in the background.
2. The batch's rows are registered under its key and "started 3 parallel agents" is said in the main thread.
3. nacelle fans out to N concurrent nested agents on the parent's own Config, posting each task's `(index, result, error, spend)` as it finishes.
4. `recordDetached` on the update loop applies each result to its row, folds the task's spend into the session total, and re-lays out.
5. The main run's `busy` flag is never touched, so the prompt stays live the whole time; a question typed mid-fan-out starts a normal run.

## What the TUI does NOT do

- No `Model.agents` map for the fan-out. No tabbed layout.
- No per-agent event routing for the parallel children. No `AgentID` envelope field (nacelle derives the nested config and streams results through the detached channel).
- The model does not wait on subagent results to finish its turn: with `Detach`, the tool reads as "started". The real results surface in the UI rows, not back in the model's context.

## How nacelle does the fan-out

- `nacelle.DelegateParallel(ctx, cfg, tasks, opts)` returns a `<-chan nacelle.ParallelTaskResult` the caller drains.
- Each task runs in its own fresh agent on `cfg`, capped at `MaxConcurrency` (default 4, max 8).
- Each `ParallelTaskResult` carries the task's index, result text or error, and its own spend.
- The channel closes when the last task finishes; an empty task list is an error, returned rather than posted.
- `NewParallelSubAgentTool` (the model-callable tool) blocks its caller until all tasks return one merged JSON — the shape the parent model consumes.

## What's NOT implemented (YAGNI)

- Tabbed multi-agent UI
- Agent worktrees, distributed file locks, Redis coordination — not a TUI concern
- `AgentID` envelope

## Files

| File | Role |
|---|---|
| `internal/tui/parallel_cmd.go` | `/parallel` parser + detached launcher + result watch |
| `internal/agent/delegate.go` | wires `NewParallelSubAgentTool` (model-callable path) |
| `internal/agent/agent.go` | `build` returns the `Config` the TUI clones for detached fan-out |
| `internal/tui/parallel_result.go` | model-callable result handling + task rows |
| `internal/tui/parallel_titles.go` | one no-tool summarizer call turning a fan-out into 6-7 word task titles |
| `internal/tasks/taskplan.go` | task-plan tooling |

## Conventions

- `[module-path]`: `github.com/FacileStudio/bulle`
- `[filet]`: `filet check .` passes; `filet test` passes
- `[events]`: no change — the model-callable path still collapses parallel results into a single tool result; the detached path uses a new `detachedResult` message on the update loop
- `[migrations/auth/muse/distribute]`: N/A
