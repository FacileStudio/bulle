# Plan — Matrix chat surface for bulle

Status: proposed, awaiting review. Written 2026-09-25 as a cold-start handoff: a reader
with no memory of the conversation that produced it should be able to execute it.

## Goal

A bulle background process holds a Matrix connection, so a message from an allowlisted MXID
runs a bulle session and the answer comes back in the same room, with end-to-end encryption
on.

## Why (evidence)

bulle can push text out but nothing can send a message in. Concretely:

- `delivery:` in a cron job accepts `""` or `file:<dir>` and nothing else —
  `validateDelivery` in `internal/agent/cronlog.go` returns `unknown delivery %q: want
  file:<dir>` for anything else, pinned by `internal/agent/cron_test.go:75`.
- Headless builds its agent with no interactive approval gate
  (`buildHeadlessAgent`, `internal/agent/headless.go:111`), so an unattended run cannot ask
  a human: whatever `approve_tools` allows is what runs.
- The only inbound surface is the IDE attachment socket, which is local-only and only exists
  for an interactive session (`--ide`, `docs/ide-protocol.md`).

Reference shape: OpenClaw and Hermes both run one long-lived process holding N thin platform
adapters, with a session store keyed per chat and default-deny authorization
(https://docs.openclaw.ai/channels,
https://hermes-agent.nousresearch.com/docs/developer-guide/gateway-internals).

## Approach

One new package, `internal/chat`, holding thin adapters, plus a `bulle chat` subcommand that
runs them and a systemd user unit so it survives logout. The Matrix adapter reuses
`mautrix-go` for `/sync` and the encrypted send path, and reuses the existing headless agent
for the actual work: an inbound message becomes a headless session keyed by room + sender,
and the reply goes back through the adapter that received it.

Sessions are keyed by chat identity, not by project. This is deliberate and fixes a real
defect: `--continue` auto-resumes the newest session **for this project**
(`cmd/flags_bind.go:90`), so two chats in one repo would interleave into one conversation.

The seam is `chat.Adapter`, a 3-method interface. Telegram, Discord and others are out of
scope for this plan but must remain cheap to add later, which is the whole reason the seam
exists rather than one hardcoded Matrix loop.

### Checked against

- filet gate: `filet.yml` (`failOn: info`, `funcLines: 35`, `fileLines: 250`, `params: 5`,
  `returns: 3`, `nesting: 4`, `complexity: 10`, `structFields: 16`, `interfaceMethods: 4`,
  `architecture.fileNamePattern: ^[a-z0-9_]+\.go$`, `maxDepth: 3`). The plan's file split is
  shaped so the code passes, not the config.
- CLI standard: `standards/cli.md` (every command carrying data gets `--json`).
- Suite release process: `ROADMAP.md` §Release (core tags `vX.Y.Z`, CI runs
  `go test ./... -race`, `golangci-lint`, `goreleaser`).
- bulle ROADMAP Track I — **conflicts, see step 9**. Track I states "Not doing — a daemon".
- Not applicable: migrations (no DB in bulle today), porte/auth (no HTTP surface), muse (no UI).

## Verified technical recipe

This is not documentation reading. Every claim below was built and run on 2026-09-25 with
`mautrix-go` v0.31.0 and Go 1.26.6, at `CGO_ENABLED=0`.

**The build requires a tag.** mautrix-go defaults to libolm through
`crypto/registerlibolm.go` (build tag `!goolm`), which imports an all-cgo package. With bulle's
`CGO_ENABLED=0` and no tag the build fails:

```
imports maunium.net/go/mautrix/crypto/libolm: build constraints exclude all Go files
```

With `-tags goolm` it selects `crypto/registergoolm.go`, a pure Go implementation of Olm.
Verified building `linux/amd64`, `linux/arm64`, `darwin/arm64` and `windows/amd64` at
`CGO_ENABLED=0`.

**The crypto works.** `CGO_ENABLED=0 go test -tags goolm ./crypto/goolm/...` passes across
account, aessha2, crypto, libolmpickle, megolm, message, pk, ratchet and session. A Megolm
round trip (outbound session, encrypt, inbound session from the shared key, decrypt) returned
the original plaintext. Account identity keys survived a persist/reload cycle through sqlite.

**The store is driver-agnostic and works pure-Go.** `crypto.NewSQLCryptoStore` takes a
`*dbutil.Database`; the cgo sqlite driver is imported only by `bridgev2/mxmain` and
`example/`, not by `crypto`. `modernc.org/sqlite` works. Two ordering traps, both hit during
verification:

1. Construct the store **before** upgrading — `NewSQLCryptoStore` registers its child upgrade
   table via `db.Child(...)` at construction time.
2. Upgrade the **child** (`store.DB.Upgrade(ctx)`), not the parent `dbutil.Database`.

That produces the 13 `crypto_*` tables including `crypto_account`,
`crypto_megolm_inbound_session`, `crypto_megolm_outbound_session`.

**Two API details that bite:**

- Read `ogs.Internal.Key()` **before** `Encrypt`. Afterwards it returns the ratcheted key and
  decrypting index 0 fails with `index earlier than our earliest known session key`.
- `OutboundGroupSession.Encrypt` refuses with `ErrSessionNotShared` unless `Shared` is set.

**Not verified, and it must be:** the full `crypto.OlmMachine` flow against a live homeserver
(device keys upload, one-time keys, `/keys/query`, decrypting a real `m.room.encrypted`
event). Verification covered the primitives and the store, not a live device. Step 1 exists
because of this gap.

## Steps (ordered)

### 1. Spike — OlmMachine against the real homeserver

Not a deliverable; a go/no-go on the only unverified assumption, before any package is
written. In a scratch directory (not the repo) with the credentials from step 0:

- create `crypto.OlmMachine`, `Load()` it, share the device keys, upload one-time keys;
- join the target room, run `/sync`, decrypt one real `m.room.encrypted` event;
- send one encrypted message and confirm it renders in Element.

If decryption of a live event fails, stop and report — the fallback is mautrix-go's
`cryptohelper` package, not a rewrite. Do not proceed to step 2 on an unverified send path.

### 2. `go.mod` — add the dependency

- `maunium.net/go/mautrix` v0.31.0 (requires Go 1.26; bulle pins 1.26 in `mise.toml`)
- `go.mau.fi/util` (for `dbutil`)
- `modernc.org/sqlite` (pure Go — bulle has no sqlite today, sessions are JSONL)

`mise run check` stays green before moving on.

### 3. `.goreleaser.yml` — the build tag

Add `flags: ["-tags=goolm"]` under `builds:`. Without it every release target fails to
compile. Verify with `goreleaser build --snapshot --clean` that all four os/arch targets
still build, since this is the pipeline that shipped `CGO_ENABLED=0` binaries.

### 4. `internal/chat/chat.go` — the seam

`Message`, `Identity`, and the `Adapter` interface: receive, send, identity. Keep it under
filet's `interfaceMethods: 4` — that ceiling is a design constraint here, not an obstacle:
three methods is genuinely enough, and a fourth means behaviour is leaking into the
interface.

### 5. `internal/chat/matrix.go` — the adapter

`/sync` loop with a persisted `next_batch` token, `m.room.message` filtering, and the
encrypted send path. Split across files if `funcLines: 35` is approached: the sync loop,
message extraction and the send path are three separate concerns and should not share a file
past the limit.

### 6. `internal/chat/store.go` — crypto state

The `dbutil` + `modernc.org/sqlite` store, at `~/.bulle/chat/crypto.db`, with the construction
and upgrade ordering from the verified recipe above. The pickle key belongs in tiroir/casier,
not beside the database.

### 7. `internal/chat/route.go` — identity, allowlist, staleness

Three jobs, one file: map a room + sender to a session key; enforce the allowlist; drop
inbound older than a configurable age and advance past it. The allowlist check happens
**before** the agent is built, not inside the run.

### 8. `cmd/chat.go` — the subcommand

`bulle chat` runs the adapters; `bulle chat channels` lists configured adapters and their
state, with `--json`. Config lives in `~/.bulle.yml` under a `chat:` key, following how
`cron:` is already read, so there is one config file and one owner.

### 9. `ROADMAP.md` — revise the daemon note

Track I currently ends: *"Not doing — a daemon, a job DB, retry, or parsing systemd/crontab
syntax inside bulle."* This plan adds a daemon. That sentence must be amended deliberately
rather than quietly contradicted. Proposed wording:

> - **Not doing in the cron track** — a job DB, retry, or parsing systemd/crontab syntax
>   inside bulle. A long-lived process for **inbound** chat is a separate concern and lives
>   in the chat track: cron stays fire-and-forget, chat is a supervised service.

Plus a new track heading for the chat surface itself.

### 10. `cmd/chat_install.go` — the systemd user unit

Follow `cercle/internal/cmd/install.go` exactly: write the unit from a Go template, generate
or read the token, `daemon-reload`, `enable --now`. Unit needs `Restart=on-failure` (not
`always` — a revoked token would restart-loop), `WorkingDirectory=`, `TimeoutStartSec`, and
`EnvironmentFile=-` for the credentials. Linger is already enabled on this machine, so the
service starts at boot.

### 11. `docs/configuration.md` and `example.bulle.yml` — document the surface

The `chat:` key, the credentials it expects, how to obtain a Matrix access token, and what
the allowlist refuses. Suite docs standard applies.

### 12. Tests

- `internal/chat/route_test.go` — session keying, allowlist refusal, staleness drop. Table
  driven, no homeserver needed.
- `internal/chat/matrix_test.go` — message extraction from a `RespSync` fixture (own
  messages, edits, reactions and membership noise all excluded), token advancement.
- The E2EE path is covered by step 1's spike; do not build a fake homeserver for it.

## Files to Modify / New

New:

- `internal/chat/chat.go` — the `Adapter` seam
- `internal/chat/matrix.go` — Matrix adapter (`/sync`, send, E2EE)
- `internal/chat/matrix_test.go`
- `internal/chat/store.go` — crypto store
- `internal/chat/route.go` — identity, allowlist, staleness
- `internal/chat/route_test.go`
- `cmd/chat.go` — `bulle chat`, `bulle chat channels`
- `cmd/chat_install.go` — systemd user unit
- `docs/plan-matrix-chat.md` — this file

Modified:

- `go.mod` / `go.sum` — three new dependencies
- `.goreleaser.yml` — `flags: ["-tags=goolm"]`
- `ROADMAP.md` — amend Track I's "not doing a daemon", add a chat track
- `docs/configuration.md`, `example.bulle.yml` — the `chat:` key

## Exit criteria

1. `mise run check` clean (`scripts/check.sh`: gofmt, vet, test, lint) and `filet check` clean
   at `failOn: info`.
2. `goreleaser build --snapshot --clean` builds all four os/arch targets with
   `CGO_ENABLED=0 -tags goolm`.
3. A message from the allowlisted MXID in an encrypted room produces an answer in that room,
   decryptable in Element.
4. A message from a non-allowlisted MXID produces no session and no reply.
5. A denied tool stays denied: the chat path does not widen `approve_tools`, and a run that
   would need approval fails closed rather than prompting into the void.
6. The bot ignores its own messages — no self-reply loop.
7. After a restart, `next_batch` and the crypto store both persist: no replay of the backlog
   and no lost decryption.
8. The unit survives logout: `systemctl --user status bulle-chat` is active after a fresh
   login.

## Risks / unknown unknowns

- **goolm is verified-working but not audited.** It compiles, cross-compiles, passes its own
  suite and round-trips correctly. I cannot vouch for its side-channel resistance. It is not
  labelled production-ready upstream.
- **The live OlmMachine flow is unverified** (see step 1). This is the single biggest risk and
  the reason step 1 gates step 2.
- **Device verification.** An unverified bot device may not be able to decrypt until verified
  in Element. Establish early whether verification is required for this room, because it
  changes the UX story.
- **A new storage dependency.** bulle has none today. `crypto.db` is new persistent state with
  a backup story nobody has written.
- **The agent has shell access to the workstation.** The allowlist is the security boundary.
  Antenne authenticates apps; bulle authorizes humans, and nothing outside bulle may widen the
  allowlist or the tool policy. Every inbound message is untrusted text.
- **One bot token, one connection.** Sharing an account or `device_id` with another process
  makes both fight. Use a dedicated account, not the Antenne sender.
- **Cross-repo release coupling.** This changes `go.mod` and `.goreleaser.yml` together; the
  tag flow in `ROADMAP.md` §Release must still be followed.

## Skip (YAGNI)

- **Telegram, Discord, or any second adapter.** The seam is the deliverable; the second
  adapter is what proves it, and that can wait.
- **The Antenne Pool as transport.** The bus is right for inbound suite events (a sonde
  incident, a CI failure) and wrong for chat: durable replay is exactly what chat does not
  want, and a personal platform identity should not sit behind a shared production service.
- **Routing chat through Antenne.** Antenne keeps the notification leg only.
- **Approval buttons in chat.** Phase 1 approvals stay local (`bulle sessions attach` on the
  workstation). Carrying approvals over chat is a security surface, not a feature, and it
  needs its own design.
- **`bulle serve` / exposing the IDE socket headless.** Defensible later; not needed to answer
  whether chat is useful.
- **A `delivery: webhook:<url>` kind.** Only needed if cron notifications should reach
  Antenne. Separate from this plan.
- **Message history in chat.** The session log stays the record; chat is a surface, not a
  store.
- **Multiple machines answering as one bot.** One daemon, on the always-on host.

## Status after implementation (2026-09-25)

Exit criteria 1, 2, 4, 6 are verified. Criteria 3, 5, 7, 8 need a live Matrix account and are
verified only by reading the code.

| # | Criterion | State |
|---|---|---|
| 1 | `mise run check` and `filet check` clean | Verified: `sh scripts/check.sh` exits 0 (gofmt, build, vet, `test -race`, golangci-lint 0 issues) and `filet check .` exits 0 at `failOn: info` |
| 2 | `goreleaser build --snapshot --clean` for four targets | Verified: linux/darwin amd64/arm64 all built, `CGO_ENABLED=0` plus `flags: [-tags=goolm]` |
| 3 | Encrypted room answers decryptably in Element | Unverified: no homeserver. The adapter, the crypto helper, the reply threading and the send path are wired and unit-tested, and the password login is proven to reach `POST /_matrix/client/v3/login` against a dead host, but no encrypted event has been decrypted or sent here |
| 4 | A non-allowlisted MXID gets no session and no reply | Verified: `Router.Check` refuses before the run and `TestRunRefusesUnlistedSenders` asserts the responder is never called |
| 5 | A denied tool stays denied | Verified by reading, and only meaningful when `approve_tools` is on: the chat path never touches `config.ApproveTools`, and `approval.Build(true)` with a nil `send` returns false from `Ask`, so the run fails closed. With the **default** `approve_tools: false` no tool call needs approval at all, so a chat run executes tools unattended — the allowlist is the only gate |
| 6 | No self-reply loop | Verified: `TestMessageFromRefuses` covers the bot's own message |
| 7 | `next_batch` and the crypto store survive a restart | Unverified: the store is passed to `cryptohelper`, which replaces the in-memory sync store, but no restart has been exercised |
| 8 | The unit survives logout | Unverified: the unit was verified with `systemd-analyze verify` only; `bulle chat install` was not run against a live config |

Answers to the open questions that the implementation settled: 3 is answered by the research
folded in below (an unverified device decrypts only while the sender's client shares keys with
unverified devices); 4 is done as written. Questions 1 and 2 are still yours: the daemon refuses
to start until the homeserver, the user id and a non-empty allowlist are configured.

Deliberately not built, and worth a follow-up rather than a surprise: bulle does not re-key a
conversation when a room is upgraded. The room ID changes with `m.room.tombstone`, the
conversation is genuinely over, and the new room is one nobody has said may reach a shell, so
bulle logs the replacement room ID and leaves the decision to the operator.

Replies thread onto the triggering message with `m.in_reply_to`, an answer too long for one
event is split across several messages with the relation on the first only, an invite is joined
(plus invite, minus remove), and withheld room keys are logged distinctly from missing sessions
so a policy refusal does not read as a bug. Self-signing is opt-in behind `chat.matrix.self_sign`
and emits a recovery key.

## Open questions for the reviewer

1. Homeserver URL, and is the bot a fresh dedicated account? (Step 0 needs this.)
2. The allowlist entry: which MXID or room ID?
3. Does this room require device verification before decryption works? (Changes step 7 and the
   UX.) Answered: an unverified device decrypts while the sender's client shares keys with
   unverified devices, which is Element's default today; the durable fix is bot-side
   cross-signing, and the spec's "do not share keys with non-cross-signed devices" recommendation
   is the cliff to watch.
4. Is the ROADMAP amendment in step 9 the wording you want, or should the daemon decision be
   recorded elsewhere first?
