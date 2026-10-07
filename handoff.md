# Handoff: Bulle Architecture Deepening

This handoff document provides context for the next agent to complete Phases 3 and 4 of the architectural refactoring plan documented in `docs/plan-architecture-deepening.md`.

## Context & Objectives

Bulle is the terminal harness for the `nacelle` Go agent SDK (`github.com/FacileStudio/nacelle`).
The architecture review identified an inverted dependency hierarchy: the core multi-turn engine and compaction pass orchestration were trapped inside `internal/tui/Model` across 118 micro-files, preventing external surfaces (`chat`, `headless`, `ide`) from sharing multi-turn state or context compaction.

The goal is to turn shallow modules into deep modules, decouple execution from the TUI, and bring full multi-turn and compaction parity to all surfaces.

## Work Completed

### Phase 1: Deep Compaction Engine (`internal/compaction`)
Compaction orchestration, prompts, and thrash management moved from `internal/tui` into `internal/compaction`:
- `prompts.go`: System prompt templates (`SystemPrompt`, `CompactAsk`, `KeepAsk`, `ConsolidateAsk`) and prompt assembly (`Prompt`, `AskWith`).
- `outcome.go`: `Outcome` struct with `Apply(conv)`, `Installs()`, and token metrics.
- `engine.go`: `Engine` holding policy, judge, agent builder, and thrash counter (`DefaultThrashLimit = 3`).
- `eviction.go`: Snapshotting, eviction viability check (`EvictionCanLandUnder`), and fallback masking.
- `runner.go`: `RunPass(ctx, conv, force)` orchestrating judge classification, forced fold checks, and LLM summarization.
- Full test coverage in `engine_test.go` and `prompts_test.go`.

### Phase 2: Core Session Engine (`internal/engine`)
Created standalone engine package decoupled from Bubble Tea:
- `conversation.go`, `conversation_tools.go`, `conversation_prune.go`: `Conversation` model maintaining `[]nacelle.Message`, managing tool call and result buffers, enforcing role alternation, and pruning orphaned tool calls.
- `events.go`: Typed streaming event definitions (`EventTextDelta`, `EventThinking`, `EventToolCall`, `EventToolResult`, `EventTurnDone`, `EventError`).
- `session.go`: `Session` struct wrapping `*Conversation` and `*nacelle.Agent`.
- `turn.go`: `Submit(ctx, prompt)` consuming `agent.Stream`, updating conversation state, and yielding typed events.
- Full unit tests in `conversation_test.go`, `session_test.go`, and `test_helpers_test.go`.

### Quality Status
- `filet check`: clean across 538 files, 58,571 lines, 0 findings.
- `sh scripts/check.sh --no-lint`: passing clean with `-race`.

---

### Phase 3: Wire Surfaces to Deepened Modules
All surfaces wired to deepened modules:
- `internal/tui`: wired to `compaction.Engine` via `m.compactor`. Pass orchestration delegates to `compaction.Engine.RunPassWithSize`. Obsolete files `compact_prompt.go`, `compact_prompt_test.go`, `compact_pass.go`, `compact_pass_test.go`, and `compact_run_test.go` removed.
- `cmd/chat`: implemented `roomRegistry` and `chatRoom` in `cmd/chat_room.go`. Persistent `engine.Session` instances keyed by Matrix room. Automatic compaction triggered before execution and on overflow error recovery.
- `internal/agent`: `headless.go`, `run_tools.go`, and `chatturn.go` wired to `engine.Session` with typed streaming events.

### Phase 4: TUI Cleanup & Micro-package Consolidation
- `internal/overflow`: folded into `internal/engine/overflow.go` (`DetectOverflow`); shallow package deleted.
- `internal/cost`: deduplicated and standardized cost formatting with `cost.FormatCost` across TUI surfaces.
- Alias zoo in `internal/tui/conversation.go`: 35 lines of type and variable aliases removed; callers fully qualified with respective packages.
- Dead code in `internal/tui`: removed orphaned structs, unused methods, and unused helpers.

---

## Mandatory Repo Invariants & Quality Gates

1. **Filet Linter (`filet check`):**
   - Code that fails filet is invalid. Run `filet check` before considering any task done.
   - Limits configured in `filet.yml`:
     - Max file lines: 250
     - Max functions per file: 8
     - Max function lines: 35
     - Max function statements: 25
     - Max struct fields: 17
     - Max interface methods: 4
     - Max directory depth: 3
2. **Code Comment Rules:**
   - NEVER add inline comments in code.
   - NEVER add comments inside a function's body.
   - Only package and exported declaration doc comments are allowed.
3. **Build Tags:**
   - Always run Go with `-tags=goolm`: `go test -tags=goolm ./...`
   - `scripts/check.sh` sets `TAGS="-tags goolm"` automatically.
4. **Git & Commit Rules:**
   - Conventional Commits: `type(scope): summary`, imperative, lowercase, no trailing period.
   - NEVER add attribution ("Generated with Claude Code", etc.).
   - NEVER add emoji in commit messages (the lefthook commit-msg hook derives it).
5. **Quality Verification Commands:**
   - `sh scripts/check.sh --no-lint`
   - `filet check`
   - `go test -tags=goolm -count=1 ./internal/compaction ./internal/engine ./internal/tui`
