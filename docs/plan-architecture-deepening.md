# Architecture Deepening Plan

Turn shallow modules into deep modules across bulle, decouple the execution engine from the terminal UI, and provide shared multi-turn session and compaction capabilities across all surfaces.

## Context and Problem

Bulle began as a terminal harness (`nacelle-tui`) for the `nacelle` Go agent SDK. Over time, it accumulated features: 3-tier semantic context compaction, parallel subagent dispatch, Matrix chat daemon, IDE socket protocol, and cron scheduling.

Because `internal/agent` imports `internal/tui` to construct `*tui.UISession`, `internal/tui` cannot import `internal/agent` without a circular dependency. Consequently, the central execution engine was implemented directly inside `internal/tui/Model`.

Strict enforcement of `filet.yml` (250 lines per file, 8 functions per file, 17 struct fields) caused `internal/tui` to fracture into 118 micro-files sharing package-private state.

Non-TUI surfaces cannot reuse this logic:
- `cmd/chat_session.go` implemented ad-hoc JSONL session files and truncates history to 50 turns (`chatHistoryLimit = 50`), bypassing the 3-tier compaction engine.
- `cmd/chat.go` reconstructs `nacelle.Agent` from scratch on every incoming message.
- `internal/agent/headless.go` supports only single-turn prompts.
- Compaction orchestration cannot be tested without full Bubble Tea `tui.Model` fixtures.

## Architectural Objectives

1. Invert the dependency hierarchy so presentation packages depend on domain engines, never the reverse.
2. Deepen `internal/compaction` so all prompt engineering, judging, pass execution, and thrash protection live in `internal/compaction`.
3. Extract `internal/engine` to manage conversation state, turn lifecycle, tool call pairing, and recovery policies independently of Bubble Tea.
4. Unify session persistence so TUI, Chat, and Headless share identical multi-turn behavior and journal formats.
5. Consolidate shallow presentation packages (`overflow`, `cost`, `status`) and remove the alias zoo in `internal/tui/conversation.go`.
6. Maintain 100% test pass rate with `sh scripts/check.sh --no-lint` and `filet check`.

## Phased Execution Roadmap

### Phase 1: Deep Compaction Engine

Target package: `internal/compaction`

Current state:
`internal/compaction` contains only slicing and data models (`Span`, `Policy`, `Fold`, `Tombstone`).
All execution logic lives in `internal/tui/compact*.go`:
- `compact_prompt.go`: Prompts (`compactSystem`, `compactAsk`, `keepAsk`, `consolidateAsk`).
- `compact_summary.go`: Summarizer calls (`summarizeInto`).
- `compact.go`: Pass orchestration (`runCompaction`, timeout handling, forced fold logic).
- `compact_light.go`: Thrash protection (`thrashLimit = 3`, `evictionCanLandUnder`).
- `compact_mask.go`: Masking fallback.

Target state:
Move all compaction execution into `internal/compaction`:
1. `compaction.Engine`:
   - Configured with `Policy`, `Judge`, and summarizer backend.
   - Evaluates whether compaction should run (`ShouldCompact`, `ShouldCompactIdle`).
   - Executes passes (`RunPass(ctx context.Context, conv []nacelle.Message, force bool) (*Outcome, error)`).
   - Manages thrash counter and fallback masks internally.
2. `internal/tui/compact*.go` becomes a thin caller:
   - Starts `engine.RunPass` in a goroutine.
   - Delivers outcome to Bubble Tea update loop to update screen status.
3. Move compaction test suites from `internal/tui` into `internal/compaction`.

### Phase 2: Core Session Engine

Target package: `internal/engine`

Current state:
`internal/tui/Model` directly mutates `m.conversation []nacelle.Message` across 195 sites, buffers tool calls, pairs tool results, catches overflow errors, and triggers retries.

Target state:
Create `internal/engine`:
1. `engine.Conversation`:
   - Encapsulates message slice.
   - Handles message appending, tool call recording, tool result closing, and turn completion.
   - Enforces role alternation and prevents orphaned tool calls.
2. `engine.Session`:
   - Owns `Conversation`, `Agent`, `Compactor`, and `ApprovalGate`.
   - Runs `Submit(ctx context.Context, text string) <-chan Event`.
   - Emits streaming events: `TextDelta`, `ThinkingDelta`, `ToolCallStarted`, `ToolCallFinished`, `TurnCompleted`, `Compacted`.
   - Automatically invokes compaction before run or on context overflow.
3. Decouple `internal/tui`:
   - `tui.Model` consumes `engine.Session` events and renders terminal views.

### Phase 3: Unify External Surfaces (Chat & Headless)

Target packages: `internal/chat`, `cmd/`, `internal/agent`

Target state:
1. Replace `cmd/chat_session.go` ad-hoc JSONL persistence with `internal/sessions` and `internal/engine`.
2. Connect `internal/chat` to `engine.Session`:
   - Matrix chat gains multi-turn tool calling, subagent delegation, and 3-tier compaction.
   - Eliminate recreating `nacelle.Agent` on every turn.
3. Update `internal/agent/headless.go` to use `engine.Session`.

### Phase 4: TUI Cleanup & Micro-package Consolidation

Target packages: `internal/tui`, `internal/overflow`, `internal/cost`

Target state:
1. Fold `internal/overflow` into `internal/engine` error recovery.
2. Consolidate `internal/cost` formatting with TUI recap helpers.
3. Remove type and variable alias blocks from `internal/tui/conversation.go`.
4. Shrink `internal/tui` file count from 118 files to focused presentation components.

## Quality Gates

Before any commit or completion:
1. `sh scripts/check.sh --no-lint` must exit 0.
2. `filet check` must exit 0 with 0 findings across all files.
