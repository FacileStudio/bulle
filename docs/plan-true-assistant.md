# Plan: a true personal assistant on Matrix

Status: approved scope, ready to build. Written 2026-10-30 as a cold-start handoff: a builder
with no memory of the conversation that produced it should be able to execute it.
Target release: `v0.86.0`.

## Goal

Bulle becomes a quiet, reliable assistant on Matrix that runs on free models:

- Three digests a day, and an interrupt only when something truly needs Yann.
- No marketing, no repeats, no leaked reasoning.
- Remembers what Yann tells it, and what it already told him.
- Asks before any write action, with a reaction or a one-word reply.
- Takes voice notes.

## Design rule: code decides, the model writes

Every model call goes to `freellmapi` with `model: auto`, a different free model each run.
Weak models leak reasoning, claim tool calls they never made, and switch language. So:

1. Plain Go fetches, filters, dedupes and decides whether to speak.
2. The model only turns a small, pre-filtered JSON list into 2-5 lines of prose.
3. Anything with consequences (sending, saving, approving) is done and confirmed by code,
   never by the model's say-so.
4. Every model output has a deterministic fallback, so a bad model run degrades to a plain
   template, not to noise.

## Decisions (from Yann, 2026-10-30)

| Topic | Decision |
|---|---|
| Cadence | Digests at 09:00, 14:00, 20:00 Europe/Paris. Interrupts only on hard triggers. |
| Quiet hours | 22:00-08:00, no interrupts; urgent items wait for the 09:00 brief. Weekends: 09:00 brief only. |
| Model | Free only. No paid profile. |
| Actions | Reads run freely. Writes need approval in chat (reaction or reply), enforced in code. |
| Memory home | `~/.bulle/memory/` (bulle-owned Markdown). Mycelium stays a search tool. |
| Memory design | Bounded `USER.md` + `MEMORY.md`, add-only daily notes, nightly consolidation with backup and veto. |
| Mail filter | Courrier classifies (`bulk`, `known`, `unknown_human`) through a new `courrier triage` call, see `docs/plan-courrier-triage.md`. Bulle adds a VIP list that wins over any class. Unknown humans go to the digest, never interrupt. Bulle keeps local sender rules as a fallback for an older courrier. |
| Triggers | Code-only poll as the primary trigger, Antenne bus as an optional extra. |
| Chat UX | Quiet notices, reactions as replies, chat commands, voice notes. |
| Language | Mirror: proactive messages in French, chat replies in the language Yann wrote. |
| Voice | Local `whisper.cpp`. |

## Evidence: what is wrong today

| Problem | Where | Effect |
|---|---|---|
| Every pulse re-reads everything with the model | `~/.bulle/jobs/assistant-pulse.yml`, `~/.local/state/bulle/cron.jsonl` | About 13 tool calls and 57-70k tokens per run, 48 runs a day, on a free tier. |
| No memory of past nudges | no state anywhere | "create landing page overdue" sent at 02:30, 03:00, 03:32, 06:30, 07:00, 07:30, 08:30, 09:00, 09:30, 10:30, 11:00 on 2026-10-30. |
| Marketing filtered by prompt | job prompts | Digests list Railway, OUIGO, BNP, LinkedIn by name. |
| Courrier cannot tell bulk from human | `Courrier/apps/api/modules/mail/imap.go` fetches only the `References` header; `schemas/email.go` has no category | Every client re-implements a guess. Fix lives in courrier (work package CX). |
| Courrier caches 100 envelopes per folder | `Courrier/apps/api/modules/mail/service.go:197` `fetchEnvelopes(..., 100)` | "People Yann wrote to" from the cached Sent folder is incomplete. Courrier builds a contacts table with a deep Sent sync. |
| No cross-account mail call | `courrier-cli` | An agent runs `courrier accounts` then one `inbox` per account. `courrier triage --all` replaces that. |
| Room chat lives in memory only | `cmd/chat_room.go` `newChatRoom` uses `engine.NewConversation()` | A daemon restart forgets the whole conversation. The session log is written, never read back. |
| Nudges are not part of the chat | `cmd/chat_ipc.go` `handleIPCSend` | Replying "tell me more" to a nudge reaches an agent that never saw it. |
| Chat runs every tool unasked | `~/.bulle.yml` `security.approve_tools: false`, `internal/agent/headless.go` `approval.Build` | A random free model has shell access and can send mail. "Confirm first" is a prompt line. |
| Inbound filter drops non-text | `internal/chat/matrixmsg.go` `textOf`, `internal/chat/matrix.go` registers `EventMessage` only | Reactions and voice notes are ignored. |
| Bot messages always notify | `internal/chat/matrixsend.go` `replyTo` uses `MsgText` | Every digest buzzes the phone. |
| Delivery failure is a log line | `internal/agent/cronlog.go` `deliverCronMatrix` | No retry, no status. |
| Antenne unverified | `antenne bus`, `antenne providers` return HTTP 404 (2026-10-30) | Cannot rely on push events yet. |

Already fixed and uncommitted in the working tree (commit in Phase 0):
fail-closed `<deliver>` delivery (`internal/agent/cronmatrix.go`), read receipts
(`internal/chat/presence.go`, `internal/chat/matrixpresence.go`).

## Research basis

- Notifications batched 3 times a day reduced stress and raised well-being. Hourly batching
  did no better than no batching at all.
  https://www.sciencedirect.com/science/article/pii/S0747563219302596
- Claude Duet: a morning brief, then messages only when something needs attention; checks
  before acting. https://claude.com/resources/articles/claude-duet
- Hermes: silence is an exact-match token, failed turns are never hidden, an empty reply to a
  human becomes a short notice, busy modes (`interrupt`, `queue`, `steer`), `/stop`, `/bg`,
  DM pairing. https://hermes-agent.nousresearch.com/docs/user-guide/messaging
- Hermes cron: durable delivery queue, `delivery_failed` kept apart from run failure, every
  missed occurrence logged. https://hermes-agent.nousresearch.com/docs/developer-guide/cron-internals
- Hermes memory: bounded `USER.md` (1,375 chars) and `MEMORY.md` (2,200 chars), frozen
  snapshot per session, writes that would overflow are refused, small models often claim a
  save without calling the tool, start a new session at natural boundaries.
  https://hermes-agent.nousresearch.com/docs/user-guide/features/memory
- OpenClaw: `NO_REPLY` and `record_result`; it removed heartbeat duplicate suppression and now
  warns repeats can be delivered. https://docs.openclaw.ai/gateway/heartbeat
- OpenClaw dreaming: nightly light/REM/deep phases, only deep writes `MEMORY.md`, a preimage
  is saved before each rewrite, a human-readable diary. https://docs.openclaw.ai/concepts/dreaming
- OpenClaw Matrix: a `quiet` streaming mode sends non-notifying notices; inbound messages are
  spooled before the sync cursor moves. https://docs.openclaw.ai/channels/matrix/messaging
- mem0 v3 moved to single-pass add-only extraction, dropping model-driven update and delete.
  https://docs.mem0.ai/migration/oss-v2-to-v3
- Anthropic: structured note-taking plus just-in-time retrieval over pre-loading everything.
  https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
- Meta Muse: per-app, per-capability access, approval before sending or paying, an audit
  trail, "forget" on request. https://about.fb.com/news/2026/09/introducing-muse-personal-ai-agent
- OpenAI Dots: a schedule must state what to check, when, which changes deserve a
  notification, and where to deliver. https://learn.chatgpt.com/docs/dots/tasks-and-memory

## Architecture

```
crontab                                   bulle chat (daemon, owns Matrix)
  */15 bulle assistant watch  ──┐           ├─ inbound: text, reactions, voice
  09,14,20 bulle assistant digest ─┤ IPC     ├─ commands (/stop /new /snooze ...) in code
  03:30 bulle assistant consolidate┘ chat.sock├─ approvals (cards, ✅ / oui)
                                           ├─ room conversations, persisted
internal/assistant  (pure Go)             └─ optional: antenne tail -> watch
  sources: courrier triage --all (classified by courrier), agenda, opus (--json)
  filter:  VIP override, fallback rules only if courrier lacks triage
  ledger:  ~/.bulle/state/ledger.json
  triggers, quiet hours, digest payload
internal/memory
  ~/.bulle/memory/USER.md, MEMORY.md, notes/YYYY-MM-DD.md, backups/
```

The model sees only the digest payload (a few items) or the chat conversation plus the
memory snapshot.

## Phase 0: contracts (one agent, before the fan-out)

Land these first so parallel agents compile against fixed seams. Small, one commit each.

1. Commit the working tree: `fix(cron): deliver only tagged output to matrix` and
   `feat(chat): send read receipts on accepted messages`.
2. `internal/settings/assistant.go`: the `Assistant` struct under a new `assistant:` key in
   `~/.bulle.yml`, merged like `chat:`. Fields:
   `Timezone`, `Digests` (map name to `HH:MM`), `QuietStart`, `QuietEnd`, `WeekendDigests`,
   `Profile`, `Sources` (`MailAccounts []int`, `Calendars []int`, `Workspaces []int`),
   `VIP []string`, `DropSenders []string`, `Language`, `Voice` (`Command string`).
   Empty values mean defaults from the decisions table.
3. `internal/chat/chat.go`: extend `Message` with `Kind` (`text`, `reaction`, `audio`),
   `Reaction string`, `Target string` (event the reaction points at), `Media` (URL and
   decryption info). Adapters that do not set `Kind` mean `text`.
4. `internal/chat/ipc.go`: extend the payload with `Notify *bool` (nil means notify) and
   `Record bool` (append to the room conversation). Add action `approve` reserved for WP5.
5. `internal/assistant/item.go`: the shared `Item` type: `Source`, `ID`, `Title`, `Who`,
   `When`, `Due`, `Status`, `Fingerprint()`. Every source returns `[]Item`.

Gate: `go test -tags goolm ./...` and `filet check` pass.

## Phase A: parallel work packages

Each package owns its files. No two packages edit the same file except where noted.

### WP1 Sources and mail filter (`internal/assistant/source_*.go`, `filter*.go`)

- Mail: run `courrier triage --all --unread --since <last watch> --json` and decode the
  contract in `docs/plan-courrier-triage.md` into `[]Item`. Courrier owns the classes
  (`bulk`, `known`, `unknown_human`) and the `counts`.
- VIP: addresses in `assistant.vip` become class `vip` before anything else, even when
  courrier says `bulk`. Only `vip` and `known` can interrupt; `unknown_human` appears in the
  digest; `bulk` appears only as a count Yann never sees unless he asks (`/status`).
- Fallback: if `courrier triage` exits 1 with the upgrade message, use
  `courrier inbox --account N --unread --json` per account and classify with built-in sender
  patterns (`noreply`, `no-reply`, `notify.`, `newsletter`, `marketing`, `mailer-daemon`) plus
  `assistant.drop_senders`. Log once that the fallback is active. Remove the fallback in a
  later release once courrier is deployed everywhere.
- Noise feedback: `MarkNoise` (WP2) on a mail item appends the sender to a local drop list
  that applies on top of courrier's class.
- Calendar and tasks: `agenda events list --calendar N --from D --to D+1 --json` and
  `opus -w N task list --json` through `os/exec` with a timeout, mapped to `[]Item`.
- Test: table tests with fixtures of the `triage` JSON (Railway as `bulk`, a person as
  `known`), a VIP override of a `bulk` row, and the fallback path with the old `inbox`
  JSON.

### WP2 Ledger (`internal/assistant/ledger*.go`)

- File `~/.bulle/state/ledger.json`, mode 0600, written atomically (temp file, rename).
- Per item key `source:id`: `first_seen`, `last_surfaced`, `fingerprint`, `snoozed_until`,
  `noise`.
- `ShouldSurface(item, now)` is true when the item is new, its fingerprint changed (status,
  due date, new message in thread), or it crossed a threshold for the first time (event
  starts within 60 min, task becomes due today, task becomes overdue). False when snoozed or
  noise.
- `MarkSurfaced(items, now)`, `Snooze(key, until)`, `MarkNoise(key)`; `MarkNoise` on a mail
  item also appends its sender to the drop list.
- Prune entries not seen for 30 days.
- Test: the 2026-10-30 transcript as a sequence: the landing-page task surfaces once, then
  stays silent across 10 watch runs, then surfaces again when its status changes.

### WP4 Persistent rooms and recorded nudges (`cmd/chat_room*.go`, `cmd/chat_ipc.go`)

- Persist each room's `[]nacelle.Message` to `~/.bulle/chat/rooms/<sha256(room)[:16]>.jsonl`
  (0600) after every turn; reload on `newChatRoom`. Keep the existing session log.
- IPC `send` with `Record: true` appends the text as an assistant message to that room's
  conversation, under the room lock, so a reply to a nudge has context.
- Daily boundary at 04:00 local: on the first message after it, compact the previous day
  into the ledger summary (existing `compaction.Engine`) and start a fresh conversation that
  carries the summary and the memory snapshot. `/new` forces the same.
- Empty reply to a human message becomes a short notice ("Je n'ai rien à ajouter." or the
  English equivalent by language), following Hermes.
- Remove `agent.ChatTurn` in `internal/agent/chatturn.go` if nothing calls it (it has no
  callers today).
- Test: write a conversation, rebuild the registry, the next turn sees the old messages; a
  recorded nudge appears in the conversation before the next user message.

### WP6 Matrix UX plumbing (`internal/chat/matrix*.go`, `internal/chat/command*.go`)

- Send proactive messages as `m.notice` when `Notify` is false; replies to Yann stay
  `m.text`. Digests and the consolidation summary are notices; urgent interrupts notify.
- Register `event.EventReaction` on the syncer; emit `Message{Kind: reaction}` with the
  emoji and target event id. Allowlist and room checks run as for text.
- Accept `m.audio` (and `m.file` with an audio mimetype) as `Kind: audio`, carrying the
  encrypted file info; drop other media as today.
- Command parser, handled in code before the model: `/stop` cancels the running turn's
  context for that room; `/new`; `/snooze <text|reply>` (default until tomorrow 09:00);
  `/quiet <hours>` (suppress interrupts); `/forget <text>` (WP7); `/status` (last runs,
  ledger size, memory sizes, next digest). Unknown `/x` goes to the model.
- `/stop` needs a per-room cancel func in `chat.Run`'s `answer`; today a turn cannot be
  cancelled. The serial queue stays (it equals Hermes `queue` mode).
- Reactions routed: `💤` on a nudge snoozes its items, `👎` marks them noise, `✅`/`❌` on an
  approval card go to WP5. The daemon keeps a map from sent event id to item keys (from the
  IPC payload, new optional field `Items []string`).
- Test: fake adapter feeding a reaction and a command; assert routing without a model call.

### CX Courrier triage (other repos: `Courrier`, `courrier-cli`)

Full plan: `docs/plan-courrier-triage.md`. Four packages (CX1 headers and category, CX2
contacts, CX3 triage and backfill, CX4 CLI) in two repos, built by their own agents. Bulle
does not wait: WP1 builds against the JSON contract and its fixtures, and falls back to local
rules until courrier ships.

### WP7 Memory (`internal/memory/*.go`, tool wiring in `internal/agent`)

- Files under `~/.bulle/memory/`: `USER.md` (cap 1,500 chars), `MEMORY.md` (cap 2,500 chars),
  `notes/YYYY-MM-DD.md` (add-only), `backups/`.
- Snapshot: at session start and at the daily boundary, inject `USER.md` and `MEMORY.md` into
  the system prompt with usage counts (Hermes format). Frozen for the session.
- Tool `remember`: one argument, a fact. Code appends `- [HH:MM] fact` to today's note and
  returns the saved line. The chat layer posts "📝 noted: ..." only from the tool result,
  so a model that skips the call cannot fake it.
- `/forget <text>`: code removes matching lines from all three files after writing a backup,
  and reports how many lines it removed.
- `bulle assistant consolidate` (nightly 03:30): reads yesterday's notes and journal entry,
  asks the model for an add-only list of durable facts and a list of `MEMORY.md` lines it
  considers stale. Code applies within the caps, writes `backups/MEMORY.md.<date>`, and posts
  a notice "Retenu cette nuit: ..." with the changes. A `👎` on that notice restores the
  backup. If the model output does not parse, nothing changes.
- Test: cap enforcement refuses an overflowing write; consolidation with a fixture model
  output applies adds, applies drops, writes the backup; revert restores it byte for byte.

### WP8 Voice notes (`internal/chat/voice*.go`)

- On `Kind: audio`: download and decrypt the media (mautrix `DownloadBytes` plus
  `attachment.EncryptedFile.DecryptInPlace`), write to a temp file, convert with
  `ffmpeg -i in -ar 16000 -ac 1 out.wav`, run `assistant.voice.command` (default
  `whisper-cli -m ~/.local/share/whisper/ggml-base.bin -l auto -nt -f {wav}`), delete temp
  files.
- The transcript goes to the room as a normal user turn, and the reply quotes it first
  ("🎙 « ... »") so Yann sees what was heard.
- Missing binary or model: reply once with the install hint, do not crash.
- Install doc in `docs/configuration.md`: build whisper.cpp, fetch `ggml-base.bin`.
- Test: a fake command that echoes a fixed transcript; assert the turn text and cleanup.

## Phase B: depends on Phase A

### WP3 Watch, digests, triggers (`internal/assistant/watch*.go`, `digest*.go`, `cmd/assistant*.go`)

Needs WP1 and WP2.

- `bulle assistant watch`: fetch, filter, ledger. Hard triggers: event within 60 min, task due
  today or newly overdue, mail from `vip`/`known` with a new message. Outside quiet hours and
  when `ShouldSurface`, send one interrupt via IPC (`Notify: true`, `Record: true`,
  `Items: keys`). No trigger means no model call and zero tokens.
- `bulle assistant digest <morning|midday|evening>`: build a JSON payload (max 8 items,
  ranked: events today, overdue, due today, known-human mail, unknown humans as a count with
  names), ask the model to write in French inside `<deliver>`, deliver as a notice with
  `Record: true`. If the output has no tag or exceeds 1,200 chars, render the deterministic
  template instead. Evening digest ends with the debrief invitation from
  `assistant-checkin.yml`.
- Weekend: only `morning`. Quiet hours: watch exits early; items wait for the next digest.
- Antenne: if `antenne status` succeeds, the chat daemon runs `antenne tail --json` in a
  goroutine and calls the same watch evaluation on relevant events, debounced 60 s. If it
  fails, it logs once and stays off. Polling stays the source of truth.
- `bulle assistant install`: writes the crontab entries (reuse `internal/agent/crontab.go`
  markers) for watch every 15 min, the three digests and consolidate; removes the old
  `assistant-pulse` and `assistant-checkin` entries after confirmation.
- Record each run in `cron.jsonl` with `status`, `surfaced`, `model_called`, and a new
  `delivery` field (`ok`, `failed`, `suppressed`). Retry a failed IPC send 3 times over
  1 minute before the file fallback.
- Test: watch with fixture sources at 03:00 sends nothing; at 10:00 sends one interrupt for a
  new known-human mail and nothing on the next run.

### WP5 Approvals in chat (`internal/agent/chatapprove*.go`, `cmd/chat_approve*.go`)

Needs WP6.

- A command classifier for `run_command`. Allowed without asking: an exact read-only
  allowlist (`courrier inbox|list|read|search|folders|accounts`, `agenda calendars list`,
  `agenda events list`, `opus workspace list`, `opus -w N task list|show`,
  `mycelium memory search`, `date`, `cat`/`ls` under `~/.bulle/memory` and the Journal dir).
  Any shell metacharacter (`;`, `&&`, `||`, `|`, backticks, `$(`, `>`, `<`) makes the command
  not read-only. Everything else needs approval. Writes to the Journal file are allowed
  (append-only, by Yann's request in the system prompt).
- Approval: post a card (`m.text`, notifies): "Je vais exécuter: `courrier send --to ...`
  ✅ / ❌". Wait for `✅` or a reply `oui`/`yes`/`ok` from an allowlisted sender, at most
  10 minutes. Anything else, a timeout or `/stop` cancels. The tool result tells the model
  what happened.
- Wire as the `nacelle.Approve` for the chat config only. Cron runs keep refusing writes
  outright (nobody to ask).
- Change the chat default: the chat surface uses this gate regardless of
  `security.approve_tools`.
- Test: classifier table (including `courrier inbox; rm -rf ~` as not read-only), approve,
  deny and timeout paths with a fake adapter.

## Phase C: integration and release (one agent)

1. Wire everything in `cmd/chat.go` and `cmd/root.go` (`bulle assistant ...`).
2. Prompts: chat system prompt adds the language-mirror rule and the memory snapshot;
   digest prompt is French-only and forbids mentioning filtered items or the checks made.
3. Docs: `docs/configuration.md` (`assistant:` key, voice install, commands, reactions),
   `example.bulle.yml`, `README.md` feature list, ROADMAP Track L, `CHANGELOG.md`.
4. Migrate Yann's setup: add the `assistant:` block to `~/.bulle.yml`, run
   `bulle assistant install`, disable `assistant-pulse.yml` and `assistant-checkin.yml`
   (`enabled: false`), restart `bulle-chat.service`.
5. Gates: `sh scripts/check.sh --no-lint`, `go test -tags goolm -race ./...`, `filet check`.
6. Ship `v0.86.0` with the `/ship` skill (tag, GitHub release, goreleaser).
7. Live check for 24 h, then confirm against the acceptance list.

## Parallel schedule

| Step | Agents | Packages |
|---|---|---|
| 0 | 1 | Phase 0 contracts (including the `triage` JSON contract, copied from `plan-courrier-triage.md`) |
| A | 6 in worktrees | WP1, WP2, WP4, WP6, WP7, WP8 |
| A (other repos) | 3 in worktrees | CX1 and CX2 in `Courrier` in parallel, then CX3 (needs both), CX4 in `courrier-cli` (starts on a fixture server) |
| B | 2 in worktrees | WP3 (after WP1, WP2), WP5 (after WP6) |
| C | 1 | integration, docs, migration, release |

Cross-repo release order: Courrier API deployed, then `courrier-cli` `v0.4.0`, then bulle
`v0.86.0`. If courrier slips, bulle ships anyway on the fallback.

Merge order inside A does not matter. Conflicts are only expected in `cmd/chat.go` wiring,
which Phase C owns; Phase A packages expose functions and leave the wiring to C.

## Repo rules every agent must follow

- `filet check` must pass: max 250 lines per file, 8 functions per file, 35 lines and 25
  statements per function, 17 struct fields, 4 interface methods, directory depth 3.
- No inline comments and no comments inside function bodies. Doc comments on exported
  declarations only, in the style of the surrounding file.
- Build and test with `-tags goolm` (libolm is not installed).
- Conventional Commits, lowercase, no emoji, no attribution lines.
- New state files are 0600, directories 0700, writes atomic.

## Acceptance

1. `courrier triage --all --unread --json` classifies Railway, OUIGO, BNP and LinkedIn mail as
   `bulk`, and bulle never names them (see the courrier acceptance list for the API side).
2. A watch run with nothing new makes no model call (`model_called: false` in `cron.jsonl`).
3. Over 24 h, no item is surfaced twice unless its fingerprint changed.
4. No message between 22:00 and 08:00.
5. No digest names a sender classed `bulk`.
6. No delivered text contains reasoning; a model run without `<deliver>` falls back to the
   template.
7. Restarting `bulle-chat.service` keeps the conversation; "tell me more" after a nudge gets a
   relevant answer.
8. `courrier send` from chat posts an approval card and does not run until ✅.
9. "remember that X" writes a line to today's note, and the confirmation only appears when
   the line exists.
10. A voice note gets a quoted transcript and an answer.
11. `💤` on a nudge silences it until the next morning; `👎` stops that sender from appearing.

Token estimate (inference, not measured): 3 digests at about 5k tokens and a few interrupts,
against 48 pulses at about 60k today, so roughly 2.9M down to under 30k tokens a day.

## Risks

- Free models may still produce unusable digests. Mitigation: the deterministic template.
- The command classifier is the security boundary for chat writes. Keep the allowlist exact
  and treat any metacharacter as a write.
- Voice transcription on CPU may take 10-30 s per minute of audio. The typing indicator covers
  the wait.
- Persisted room files hold private mail subjects and journal text: 0600, and listed in
  `/status` so Yann knows they exist.
- Antenne returned HTTP 404 on 2026-10-30; it stays optional until verified.

## Open items for Yann

- The VIP list (names or addresses that should always interrupt).
- How the Courrier server is deployed (the repo has no release tags), so its agent can ship.
- Whether the Antenne instance should be fixed as part of this, or left optional.

Resolved: the Sent folder is already exposed (`courrier folders --json` has `"type": "sent"`),
and courrier will build its own contacts table, so bulle needs no sent-folder cache.
