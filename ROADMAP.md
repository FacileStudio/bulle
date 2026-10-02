# bulle Roadmap

This file tracks UI-only work. Core SDK changes live in `../nacelle/ROADMAP.md`. Both repos release with the same tag (e.g. `v0.8.0` / `tui/v0.8.0`) in a two-commit flow: core first, then UI pinned to it.

---

## Track D — Legibility (UI only)

1. **Tool-call line summarizer & aggregation** — `view.go` `absorb` replaces raw-input `say` with name + primary argument (`command`, `path`, `pattern`, `query`), truncates via `truncate`, and aggregates consecutive calls. *Done: kind-based grouping in `toolgroup.go`, batch duration and deduplication across failures in v0.21.6/v0.21.7.*
2. **Silent successes** — successful tool result prints nothing; duration folds into call line. Failures keep loud line; repeated identical failures collapse to one line with count. *Done via `failureCollapse`.*
3. **Refuse malformed tool-call JSON** — duplicate keys at trust boundary surface as refused call with reason, not decoder-last-wins. *Done via `strictObject` in `toolline.go`.*
4. **Collapsed thinking** — `thinking.go`: `KindThinking` renders one dim line while running, then `· thought for Ns (ctrl+t to read)`; ctrl+t toggles viewport over `m.run.reasoning`. *Done.*

---

## Track E — Claude-Code-grade Organisation (builds on D)

5. **Batching by kind** — consecutive same-kind calls render as one updating line (`⏺ 4 commands · mycelium sync · ls docs · …`), flushed on kind change or `settle`. Clamped within pane width to avoid live region wrapping. *Done in `toolgroup.go`.*
6. **Status footer** — elapsed timer, input/output/cached tokens, ctx, trimmed, and cumulative cost on status line. *Done via `statusrender.go` footer and `/status` command.*
7. **Turn boundaries** — spacing plus running totals at `KindTurn`. *Done via `turn.go` / `view.go` in v0.21.7.*
8. **Recap before quit** — tools run, tokens, cost, two lines printed after `Run` returns beside exit-transcript dump. *Done via `recap.go`.*

---

## Track H — Sessions, Then Compaction

- **Log rotation & write-failure warning** — rotate session files at 256 KB, gzip old file (`.gz`), start new timestamped file. *Done via `sessionrotate.go`.*
- **Session summary command** — *Done via `/status` command.*
- **Dynamic compaction window** — automatically scales the trigger when unconfigured. *Done: the ratio ladder in `docs/plan-compaction-robust.md`. Phase 0 shipped the config surface and `ResolveBudget`; Phase 1 landed the zone/ledger model in `internal/compaction` (`Plan`/`Apply`/`Tombstone`) and the tiered trigger — soft tombstones history with no model call, mid and hard folded it into one `[state ledger]` under a pinned anchor. Phase 2 added the opt-in TypeSafe System One judge (`internal/jev`, batched keep/prune/ledger, hard force-summarizes what it keeps). Phase 3 surfaced it: the footer shows the live ratio and tier (`↕120k/200k · 0.60 · soft`) and `/status` reports the ledger and the last pass's tier, with ratios, tiers, judge and privacy written up in `docs/configuration.md`, `README.md` and `example.bulle.yml`. Phase 4 hardened it: re-running the whole validation matrix closed a hole in I1 where the ledger could absorb a kept tool call and orphan its result — `internal/compaction` now extends the ledger zone over its own replies and carries an absorbed block forward (`Apply`, `Plan`, `ledgerCarry`). 2026-09-22 followed a review against published practice: the verbatim tail is sized by `keep_tokens` with `keep_turns` as its floor, the ladder is read against the window less a `reserve_tokens` runway with `window_tokens` to override a backend that reports none, and the pre-send guard fires at the trigger itself — the footer's denominator is that usable window (`↕120k/160k · 0.75 · soft`).*
- **Summarize near the limit** — the gap nacelle's own Track H item 17 left open: past the mechanical drop of old tool results, an assistant-text-heavy history had no fallback before the window ran out. *Done here, consumer-side as that item requires: smart summarizes the history into the ledger once the ratio is crossed, keep the deterministic tombstone as the failure path, and let `/status` name the tier that ran (`internal/compaction`, `internal/tui/compact*.go`).*
- **Resume** — `--continue` picks the newest session under `~/.bulle/sessions/<project>/`; `/resume` picker in the TUI to resume past conversation. *Done via `--continue` flag and `/resume` command.*
- **Subagents overview** — show list of running subagents and current task progress one per line under the input prompt (like pi or antigravity). *Done via `parallel_result.go` in v0.26.0.*

---

## Track I — Background scheduling (cron)

A cron for agents: unattended, no daemon. Jobs live inline in `~/.bulle.yml` under `cron:`; the `bulle cron` subcommand fronts the existing headless path. The scheduler is systemd/Cron — bulle only surfaces and arms it.

- **`bulle cron list`** — show jobs and their armed state. *Done.*
- **`bulle cron run <name>`** — run one job headless, deliver the transcript. Defaults are reversed for unattended runs: shell (`commands`) off and `enabled` off, because a run nobody can answer must not reach a live approval prompt. `install` refuses a disabled job so test-run-first is explicit. *Done (Phase 1).*
- **`bulle cron install <name>`** — directly install the job into user's crontab with tagged markers, log redirection, and automatic PATH/key resolution. *Done.*
- **`bulle cron uninstall <name>`** — remove an installed job from crontab. *Done.*
- **Delivery** — `delivery: "file:<dir>"` appends a status header + transcript to `<dir>/<name>.log`; unset means journal/stdout only. *Done (Phase 1).*
- **Phase 2 (not yet built): promotion UX** — repeat a chat job, agent offers to schedule it, test-runs it once into the same thread, creates it enabled-by-design, and auto-disables on failure with a notification. Mirrors the `syntheses/background-agent-scheduling.md` reference.
- **Not doing in the cron track** — a job DB, retry, or parsing systemd/crontab syntax
  inside bulle. A long-lived process for **inbound** chat is a separate concern and lives
  in the chat track: cron stays fire-and-forget, chat is a supervised service.

---

## Track J — Diagnostics loop

The model should see what its edits broke without being told to check: filet findings land in the session right after an edit, and a pull tool re-checks on demand.

- **Post-edit diagnostics loop** — run `filet check` on every file a tool just wrote, injecting the findings into the session through an AfterToolCall hook (built-in, `internal/diagnostics`, kill switch `tools.diagnostics`), plus a `diagnostics` pull tool the model can call between edits. Design source: `syntheses/coding-agents-lsp-integration.md`. *Done via `internal/diagnostics` and `withDiagnosticsHook` in `internal/agent`.*

---

## Track K — Chat

An inbound surface. `bulle chat` runs one long-lived process holding thin platform adapters,
so a message from an allowlisted identity starts a session and the answer returns in the same
room, with Matrix end-to-end encryption on. This is the daemon Track I declined, held
deliberately apart from cron: cron stays fire-and-forget, chat is a supervised service.
Design and evidence live in `docs/plan-matrix-chat.md`; the settings are in
`docs/configuration.md`.

- **One process, N thin adapters** — `internal/chat` holds the seam, and the Matrix adapter
  (`/sync` plus the encrypted send path) is the first one. Telegram, Discord and the rest
  stay cheap to add rather than built now.
- **Sessions keyed by chat identity, not by project** — `--continue` resumes the newest
  session for a project, so two chats in one repo would interleave into one conversation;
  the chat's key is the room instead.
- **Default-deny allowlist** — which MXIDs may start a session and which rooms are read at
  all. Both empty refuses every message, and the check runs before the agent is built.
- **`bulle chat install`** — writes and enables the systemd user unit, so the process
  survives logout; the unit restarts on failure, not always, so a revoked token cannot
  restart-loop.
- **Not doing** — a second adapter (the seam is the deliverable and a second platform is what
  proves it), routing chat through the Antenne Pool (Antenne keeps the notification leg
  only), approval buttons in chat (approvals stay local on the workstation), message history
  as a store (the session log is the record), or several machines answering as one bot (one
  daemon, on the always-on host).
- **Risks** — `goolm`, the pure-Go Olm implementation the `CGO_ENABLED=0` release needs, is
  verified working here but not audited upstream; an unverified bot device may not decrypt
  until it is verified in Element; the crypto database is new persistent state with no backup
  story; and the allowlist is the security boundary, because the agent has shell access to
  the workstation.

---

## Release process

1. Core lands changes, tags `vX.Y.Z`.
2. UI bumps `go.mod` to require that core version, applies UI changes, tags `tui/vX.Y.Z`.
3. Both CI pipelines must pass (`go test ./... -race`, `golangci-lint`, `goreleaser`).
