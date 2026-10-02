# Changelog

## [Unreleased]

### Changed

- refactor: rename the project from `kori` to `bulle` — binary, module path, config directory (`~/.bulle.yml`), environment variables (`BULLE_*`), install script, Homebrew cask, GoReleaser project name and every remaining `kori` reference in docs and tests (`main.go`, `cmd`, `internal`, `docs`, `README.md`, `ROADMAP.md`, `filet.yml`, `.goreleaser.yml`, `install.sh`, `.gitignore`)

## [0.79.0] - 2026-09-25

### Added

- feat(chat): send typing indicators while generating turn responses (`internal/chat`, `filet.yml`)

## [0.78.0] - 2026-09-25

### Added

- feat(chat): inbound chat surface over Matrix with end-to-end encryption (`bulle chat`). Runs a supervised daemon holding a Matrix connection, turning allowlisted inbound messages into headless agent turns and returning threaded answers into the room. Supports device verification via `bulle chat verify --recovery-key`, cross-signing key generation, room invite auto-joining, and Megolm group session management for encrypted rooms (`internal/chat`, `cmd`, `internal/settings`, `docs/configuration.md`)
- docs(ide): document the surface for users, and be honest about diff (`README.md`, `docs/`)

## [0.77.0] - 2026-09-24

### Added

- feat(ide): a session can publish itself to an editor over a unix socket — `--ide`, or `$BULLE_IDE` for every session in a shell. It writes `~/.bulle/ide/<pid>.json` (0600 in a 0700 directory, removed on exit) and listens on `$XDG_RUNTIME_DIR/bulle/<pid>.sock`, newline-delimited JSON, one object per line, every object carrying a protocol version and a type. The frozen contract is `docs/ide-protocol.md` and its first consumer is `bulle.nvim`. The session reports the run's shape (`hello`, `turn`, `done`), what it is doing (`tool`, `edit`, `error`), and the one thing that needs an answer (`approval`); the editor can answer an approval, run a prompt with its cursor's place attached, ask the session to scroll to a file, and stop a run. The `edit` event carries the changed line span in the new file plus the added and removed counts, so an editor marks a change without re-deriving it or reading the file. Forward compatibility is mandatory on both sides — a receiver ignores an unknown type and unknown fields inside a known one — so a protocol version bump is never a breaking change. A session that publishes to nobody pays nothing for it: with neither the flag nor the variable there is no socket, no file, no goroutine and no hook, and the nil surface costs a nil check (`internal/ide`, `internal/tui`, `internal/agent`, `cmd`, `docs/ide-protocol.md`)
- feat(tui): the stats line under the state line now opens with `provider · model` — `openrouter · deepseek/deepseek-v4.1-flash` — ahead of the price, in the banner's own words, so which model is being billed is readable mid-run rather than only at launch. It follows a `/model` switch, and it names the *resolved* model: a config that leaves `provider.model` empty lets the backend supply its own default, and the banner and the session config now read one shared helper for it, so the two cannot disagree. The session total, the token pair and the context figure now come from `total()`/`tokenTotals()` for the footer, `/cost`, `/status` and the recap alike, instead of four hand-written sums (`internal/agent`, `internal/tui`). Incidental: `/status` no longer prints an empty `anthropic/` for a session that never named a model

### Fixed

- fix(ide): an approval is refused only when an editor was actually asked. The socket's first cut read "nobody attached" as a refusal, so `--ide` — or a `BULLE_IDE=1` left in a shell profile — denied every tool call the moment it was asked, with the terminal prompt still on screen asking a question that had already been answered. The answer is now three-valued: with no editor attached nobody was asked, and the terminal decides, which is where that decision sat before the socket existed. An editor that *was* asked and then went quiet, lost its connection, or ran out the clock is still a refusal, never an implicit yes (`internal/ide`, `internal/tui`)
- fix(ide): a session takes its socket and its discovery file down with it. Both were left on disk when a `--ide` session exited, so the next reader found a file naming a socket nobody was listening on and had to fall back to the pid-liveness check the protocol keeps for a killed process (`internal/agent`)
- fix(tui): the context counter was sized from the *run* total instead of the conversation. `KindDone` carries every turn's usage summed — each turn re-bills the whole conversation, so that figure is a bill, not a size — and sizing from it showed a three-turn run as three times the context it held (201k against a conversation of 101k), pinned in `TestTheContextSizeIsTheLastTurnsInputNotTheRunsTotal`. Compaction reads that figure and selects its tier from it, so the inflation also compacted early on every multi-turn run. Only a turn sizes now (`internal/tui`)
- fix(tui): the live price estimate carried the cost-per-token rate it had learned from the last turn across `/clear` and across `/model`. After a clear it priced a session that had spent nothing, putting a phantom dollar figure on the next stream; after a switch it priced the new model's tokens at the old model's rate until a turn reported its own cost. Both drop it now, and every model switch goes through one `activate` that writes the active-model fields and the usage sink in one place, instead of three call sites repeating the same five lines (`internal/tui`)
- fix(cost): raise nacelle to v0.28.2. Its OpenAI-schema backends (`openai`, `openrouter`, `google`) reported a prompt token twice whenever any of it was cached or written: `prompt_tokens` is the whole prompt and its `prompt_tokens_details` parts are parts *of* it, not neighbours of it — a 34375-token prompt with 32768 written read as 67145 against a `total_tokens` of 34377. Since the context is sized as `InputTokens + CacheReadTokens + CacheCreationTokens`, the figure was up to double on those backends, and compaction ran early. v0.28.1 subtracted the cached share and v0.28.2 the written share as well, so `Usage.Total` equals the provider's own `total_tokens` on every backend (`go.mod`)

## [0.76.0] - 2026-09-24

### Changed

- feat(compaction)!: the tier ladder is two rungs instead of three. `limits.compaction.hard_ratio` is **removed**, and `limits.compaction.mid_ratio` is **renamed to `smart_ratio`** — the old name is refused at load by the strict decoder rather than silently ignored, and under the environment layer `BULLE_COMPACTION_MID_RATIO` and `BULLE_COMPACTION_HARD_RATIO` are now refused too, since that layer would otherwise ignore them and quietly run on the default. Migrating is deleting one line and renaming the other; there is nothing to re-tune, since the forced fold is now derived rather than configured. Whether a pass *forces* is no longer scheduled by that second ratio: it is derived, from whether the fold the judge produced would leave the conversation under its trigger (`compaction.LandsUnder`, `compaction.Fold.Forced`). A fixed ratio was a proxy for that question and a coarse one — two conversations at 0.81 and 0.89 of the usable window ran the same pass, though only the second needed forcing — and it drifted on large windows, because the reserve is capped at 64k and stops growing above a 320k window: the old `hard` rung sat at 72% of the raw window at 200k but 84% at 1M, later than the 70–75% the field converges on. What forcing costs is bounded but real: it never drops a block, since a `prune` still needs the judge's probability and confidence — but the blocks it overrides are exactly the ones the judge said must survive verbatim, and they go to a summarizer whose prompt says to be as short as correctness allows, so a fact in one can be lost to summarization. That is why the decision is made on an upper bound of the rebuilt size rather than a guess. The judge is no longer asked to force either — `JudgeRequest.Force` is gone, and forcing is one lever pulled after the verdicts rather than a parameter to the question that produced them (`internal/compaction`, `internal/settings`, `internal/tui`, `internal/agent`)
- docs(compaction): the `limits.compaction` block in `~/.bulle.yml` now says which settings are a decision and which are a default — `judge.enabled` and `compact_at` are the two that are yours, and the ratios carry the rule that the soft tier is free and the smart tier is a call, so raise `soft_ratio` before touching `smart_ratio`. No value changed (`internal/settings`, `README.md`, `docs/configuration.md`)

### Fixed

- fix(settings): `TestShowHooksFromEnv` and `TestSetupAgentCustomTools` resolved settings through the real chain and so read the developer's own `~/.bulle.yml` and whichever profile it names — an unrelated key in someone's config could fail the suite, which is exactly how the `hard_ratio` removal surfaced. Both now run on a home of their own, the way their siblings already did (`internal/settings`, `internal/agent`)

## [0.75.0] - 2026-09-24

### Added
- feat(settings): `provider.api_key_command` and `limits.compaction.judge.api_key_command` — a key can be the output of a command instead of the value of a file, so a config file, a profile or a dotfiles repo can carry no secret at all: `api_key_command: "tiroir get OPENROUTER_API_KEY"`, run through `sh -c`, its trailing newline trimmed, its stdin closed so a prompt cannot seize the terminal, killed after 10 seconds. It is a source and not an override — a key from a flag, the environment, a literal `api_key` or a profile wins, and the command fills only what is empty — so an exported `TYPESAFE_API_KEY` keeps working untouched and a judge command never runs while one is set. The command runs only where a key is used, when a session builds its provider or its judge, so inspection commands (`bulle list`, `bulle sessions`, `bulle cron list`) never run it and a locked secret store cannot break them. A command that fails, times out or prints nothing is refused at startup naming the field, never left as an empty key, which would reach the backend as "no credential" and read exactly like a config that forgot one (`internal/settings`, `internal/agent`, `internal/tui`)

## [0.74.0] - 2026-09-23

### Added
- feat(settings): `limits.compaction.keep_turns` is refused at load when the floor it names alone exceeds the half of a pass the tail may fill, which it previously overrode silently: a generous floor pinned the window open above the trigger, so no pass could move it, because the active window is never rewritten. The check fires only where the cap is decidable at that layer, a written `window_tokens` or `compact_at`, since with neither the window is the backend's own and refusing would judge a number nobody wrote. The floor is weighed with the package's existing one-block bound rather than a second estimate (`internal/settings`, `internal/compaction`)

### Fixed
- fix(compaction): a consolidating pass now folds the history instead of trusting the judge's keep verdicts, so the rewrite it asks for is measured against turns no earlier pass compressed. A keep-heavy classification used to leave the pass with no ledger block to fold, and those turns are what carry the earlier ledger into the ask: the summarizer was never called, the consolidating addendum never reached the model, and a body past `MaxLedgerTokens` stayed over budget for the rest of the session (`internal/tui`)
- fix(compaction): the ledger's identity is claimed by a marker a reader cannot type. The sentinel now ends in the private-use codepoint U+E000, because matching the whole first line (the 0.73.0 fix) was not enough on its own: a turn whose first line spells the marker the way a document prints it still matched, and identity decides whether a message is folded into or pruned away. `ledgerIndex` takes the first match in the `anchor..active` range, so such a turn took the ledger's zone while the ledger the package wrote read as history behind it, breaking I3a by ordering alone. Binding identity to the assistant role is not available, since assembly picks the ledger's role as the opposite of its neighbour and an assistant-anchored session is folded into a *user*-role ledger; neither is recording it outside the message text (`nacelle.Message` is a role and a list of parts, and `Part` is sealed to `nacelle`). The marker being untypeable is therefore the discriminator. Nothing on disk is orphaned by the change: a resumed session is rebuilt from its question and answer lines alone (`internal/sessions`), never from the ledger (`internal/compaction`)

## [0.73.0] - 2026-09-23

### Added
- feat(compaction): a run the provider refuses for context length now compacts and retries once, instead of failing with the request still too long. The refusal is recognised across the providers' own wording (`internal/overflow`), held quietly rather than printed, and answered by a *forced hard* pass followed by the run being started again on the compacted conversation — once per turn, and not at all with `compact_at: 0` (`internal/tui`)
- feat(compaction): a ledger body past its budget (2000 tokens, the same ceiling one summary is written under) is *consolidated* — one rewritten block instead of an addition — which is the only thing that lets a ledger that otherwise only ever grows get smaller again (`internal/compaction`, `internal/tui`)
- feat(compaction): `limits.compaction.keep_tokens` — the verbatim tail is now sized by a token budget (40k by default) instead of a count of messages, so two heavy file reads can no longer hold the window open just by being the newest turns. `keep_turns` drops to 1 and becomes the *floor* under that budget: the live turn alone, the one turn no summary may stand in for. Neither bound may take more than half of what a pass may fill, since a tail that size could not land under the trigger it fires (`internal/compaction`)
- feat(compaction): `limits.compaction.reserve_tokens` — a runway held back from the context window for the model's own answer (a fifth of the window within 8k–64k when unset). The ratio ladder is now read against the window *less* that reserve, so `hard_ratio: 0.90` leaves the reserve plus a tenth of the usable window for the response instead of a tenth of the raw one
- feat(compaction): `limits.compaction.window_tokens` — an explicit override of the backend's reported context window, for the gateways that under-report one and the runners that report none at all (which otherwise leave every session on one global `75000`)
- feat(tui): `/status` names the window the ladder is measured against — `window · 200k raw, 160k usable, 40k reserved for the answer` — and the footer's ratio is the load against that usable window, so the figure and the tier can no longer disagree

### Changed
- fix(compaction): the ledger no longer grows without bound, nor doubles itself on every pass. The old fold handed the previous ledger to the summarizer and concatenated whatever came back unless it textually contained the old body, so a model that *reworded* what it was shown produced `previous + reworded previous + new` — the body doubled a pass and nothing measured it, because the never-grow check compares whole-conversation estimates. The fold is now a structural merge (per section, line by line, on a normalized key) and the prompt says plainly why the earlier ledger is shown: so it is not repeated. The ledger's one rewrite is a consolidation, gated on the rewrite still naming every identifier the old body named (`MissingIdentifiers`), and a rewrite that would drop one is refused and the merge stands — growth, which the next pass can attack, rather than a fact nobody can recover (`internal/compaction`, `internal/tui`)
- fix(compaction): I3 is restated in three parts rather than one, because the single sentence was already violated in letter while the property it protected held: the ledger is never a judge block, never history and never a prune candidate (I3a); no summarizer call is ever handed the ledger *alone*, so a rewrite is always measured against turns no earlier pass compressed (I3b); and the body is never silently rewritten (I3c) (`docs/plan-compaction-robust.md`)
- feat(compaction): the pre-send guard now compacts at the trigger itself rather than `compactSlack` above it. The trigger is already the soft ratio of the window a turn can fill, so waiting no longer bought headroom — it only changed which tier answered, and upward. The thrash guard's "did the pass land" test moved to that same figure: a margin between the two reset the count on a size the next send compacts again, so a session in that band fired a full pass on every send with nothing said. `compactSlack` is now a fixture offset in the tests only (`internal/tui`)
- fix(compaction): the byte measure counts what a request actually carries. A tool call's own arguments — often the largest thing in an edit or write turn — counted as nothing, and a `Reasoning` block counted in full although every backend drops it when it builds a request. Both fed the freed-token report, the never-grow check and `evictionCanLandUnder` (`internal/compaction`)
- feat(settings): `keep_turns`, `keep_tokens`, `reserve_tokens` and `window_tokens` are validated at load, and the last three are threaded through the environment as `BULLE_COMPACTION_KEEP_TOKENS`, `BULLE_COMPACTION_RESERVE_TOKENS` and `BULLE_COMPACTION_WINDOW_TOKENS`

### Fixed
- fix(settings): the `limits.compaction` guards were written as range tests, which are false for NaN, and `strconv.ParseFloat` accepts `nan` while the YAML resolver accepts `.nan` — so `hard_ratio: .nan` loaded clean and, through `int64(NaN*window+0.5)`, pinned every message at the hard tier: a judge call and a summarizer call on every turn, forever, with the config reading as valid. The guards now reject NaN. A negative `compact_at` is refused too, rather than silently read as "off" by every gate that tests for a positive ceiling, which took overflow recovery with it (`internal/settings`, `internal/agent`)
- fix(compaction): the thrash guard counted a pass as "did not land" only above the trigger plus a margin, while the guard that fires a pass acts at the trigger itself — so a pass stopping between the two cleared the count, the guard never stood down, and a full pass fired on every send with nothing said. Both now compare against the same figure (`internal/tui`)
- fix(compaction): the ledger sentinel was matched by prefix, so a message that merely opened with the marker was read as the ledger and the real one became a prune candidate, breaking I3a. It is now matched as a whole first line, and the package's own ledger identity is pinned by a test either way (`internal/compaction`)
- fix(compaction): the selectors trusted a span that reached past the conversation, which is a process-killing panic the moment a plan and a conversation disagree; they now trim instead. An empty-body ledger — a shape the package installs on purpose — is no longer dropped in place of a role repair (`internal/compaction`)
- fix(compaction): `foldOrphanResults` was untested, so the guard that keeps a tool pair whole at the history boundary could have been deleted with the suite staying green; it now has a test on the shape it exists for, and so do I3a and I3b (`internal/compaction`, `internal/tui`)
- fix(compaction): a job's `compaction` block was validated before it merged over the session's, so a usable job could combine into a ladder that cannot tier with nothing re-reading the merged result (`internal/agent`)
- fix(compaction): the judge calibration harness ran a live, billed call inside `go test ./...` whenever a credential happened to be in the environment. It is now behind an explicit `BULLE_CALIBRATION=1` opt-in as well as the key (`internal/compaction`)
- fix(tui): overflow recovery ignored an aborted turn and could retry and answer it anyway; and the pass snapshot shared the `Parts` arrays it read, leaving its isolation resting on a comment rather than on the code (`internal/tui`)
- fix(settings): `FromFlags` declared its names on the process's flag set on every call, so `go test -count=N` died on a redefined-flag panic and only `-count=1` passed. The declaration is now memoized once per process (`internal/settings`)

## [0.72.0] - 2026-09-22

### Added
- feat(compaction): a labeled-example calibration harness for the judge — one batched call over a corpus of labeled history blocks, re-scored across a sweep of prune thresholds, printing where the model's confidence and its accuracy part company (`TYPESAFE_API_KEY=... go test ./internal/compaction -run JudgeCalibration -v`; skipped without a key, so CI never pays for it)

### Changed
- feat(compaction): `limits.compaction.judge.prune_threshold` defaults to `0.75`, not `0.85`. The old number had never been measured against the model; the harness shows it recovering 17% of the blocks a careful operator would drop where 0.75 recovers 50%, at the same zero false prunes, and lifts agreement with the labels from 65% to 76%

### Fixed
- fix(compaction): the pre-send guard did nothing at all on a backend that cannot count tokens. It read `if count, err := CountTokens(...); err == nil && count > trigger` — and the OpenAI-compatible runner answers with `*Unsupported`, so the error branch skipped the whole check while the post-turn trigger kept running and compaction still looked healthy. A conversation already past the ceiling was sent anyway and the overshoot was absorbed into the next turn instead of prevented; the guard now falls back to the last usage-reported size, so the two automatic triggers agree on what the context costs (`internal/tui`)
- fix(compaction): a usage event reporting no input tokens erased `m.size`, the only measure both automatic triggers read, silently standing compaction down until a real usage arrived — a backend that reports usage on some events but not others was enough to switch it off. A zero is now read as "unknown" and the last real reading kept (`internal/tui`)
- fix(compaction): the judge's choice questions were rejected by the live endpoint — TypeSafe takes a choice's `criteria` as an object keyed by option, not a list, so every classification was a 422 and fell back to the mask. The judge had never successfully classified anything, and no test could see it because they all talk to a stub (`internal/compaction`)
- fix(compaction): every question in a batch was identical, so the model had no way to tell which block it was being asked about and answered the same way for all of them; each question now names its own block in its instructions (`internal/compaction`)
- fix(compaction): the criteria described a dead end under both `ledger` and `prune`, leaving the model no confident answer — the distribution flattened under the confidence floor and no verdict was ever acted on. The three descriptions are now disjoint, and confidence rises from 0.2–0.3 to 0.8–0.9
- fix(compaction): the soft tier tombstoned however little it found, rewriting the prompt-cache prefix for a few hundred bytes; it now stands down below `MinCleared`, and `DroppableBytes` is the pre-flight a caller measures a pass with before paying for the cache it invalidates
- fix(compaction): the judge's request was bounded in block count but not in bytes, so a handful of large tool results produced a multi-hundred-KB call to a small decision model; each block is now cut to `maxBlockText`, the goal to the same cap — it is the anchor's own text, which a pasted first message can make larger than every block together — and the batch to `defaultMaxState`, folding the oldest blocks that do not fit rather than asking about none of them
- fix(compaction): a tombstoned result's placeholder never closed its bracket — `[dropped 40000 bytes. Re-run the tool…` where the documented shape is `[dropped N bytes]`
- fix(compaction): the versioned model id that answered was decoded and thrown away, and the setting defaults to the drifting `jev-latest` alias; `/status` now names the model that answered and what it billed, so the setting can be pinned to the build the thresholds were tuned against (`internal/compaction`, `internal/tui`)
- refactor(compaction): a pass runs on a snapshot taken on the update loop instead of reading the model off its own goroutine, so its read safety is local rather than an invariant spread across every command that might mutate the session mid-pass (`internal/tui`)

### Removed
- chore(compaction): the dead `alignedEvictCut` wrapper, whose only caller in the repository was its own test, and the superseded `compaction-upgrade.md` spec at the repository root

## [0.71.0] - 2026-09-21

### Added
- feat(compaction): ratio-based context compaction with a soft/mid/hard ladder, a pinned anchor and a persistent `[state ledger]` that folds new facts into itself instead of re-summarizing (`limits.compaction`, `BULLE_COMPACTION_*`)
- feat(compaction): an opt-in TypeSafe System One judge (`limits.compaction.judge`, `TYPESAFE_API_KEY`) that classifies each history block keep/prune/ledger before the ledger is written; off by default because enabling it sends conversation history to a third party
- feat(tui): the status line shows the live context ratio and tier (`↕120k/200k · 0.60 · soft`), and `/status` reports the accumulated ledger's size, the last pass's tier, and whether the judge is on — the one setting that sends history off the machine is otherwise invisible once enabled

### Changed
- feat(settings): `limits.compact_at` is unset by default and the ceiling now derives from the compaction ratios against the backend's context window; an absolute value still overrides and `0` still disables compaction
- feat(settings): `limits.compaction` ratios and `judge.prune_threshold` are validated at load — a ratio outside `(0,1]`, a ladder with its rungs out of order, or an unusable prune threshold is now a startup error instead of a trigger that silently never fires

### Fixed
- fix(compaction): a pass can no longer orphan a tool pair the ledger absorbed — the ledger zone claims the replies to its own calls and carries an absorbed block forward, so a later prune cannot split a call from its result and a later rebuild cannot shed what a fold put in (`internal/compaction`)
- fix(compaction): a judged pass that comes back with an empty summary masks instead of folding away the turns the judge tagged for the ledger, so a provider that streams no text can no longer drop history silently and call it a summary
- fix(compaction): a rebuild heavier than the conversation it replaces is refused and the context left unchanged, so a pass never grows the context it exists to shrink (folding a two-byte turn can no longer install a verbose ledger)
- fix(compaction): the pinned head is extended over the replies answering the calls it carries, and a pass that folds nothing still stands a ledger between the head and the active window — the two cases where the assembly could merge a kept turn into the anchor or emit two messages of one role
- fix(jev): a transport error (a dropped connection or a per-attempt timeout) is retried like a 429/529, bounded by the caller's context rather than spending the attempt budget on requests that are already out of time
- fix(compaction): the soft tier tombstones tool results only — every backend drops a `Reasoning` block when it builds a request, so stubbing one freed no context while taking the chain of thought out of a transcript the reader can still scroll back to, and the pass reported the saved bytes as if it had bought headroom
- fix(compaction): a judge built without a prune threshold falls back to the shipped `0.85` instead of pruning on any probability at all, and a block the model kept can no longer be dropped by a threshold of zero
- fix(compaction): `soft_ratio: 0` no longer turns compaction off while the tier ladder still reports it on — the derived ceiling falls back to `75000` instead of zero, which every trigger reads as "disabled", and the setting is refused at load
- fix(compaction): a pass refuses a plan that no longer covers the conversation instead of indexing past the end of it, so a `/clear` or `/resume` landing mid-pass cannot take the session down with it, and the mask fallback refuses one too rather than promising a fallback it did not run
- fix(compaction): a tool call's own arguments (abbreviated) reach the judge, which was classifying every block from the tool's name alone
- fix(jev): a gateway's 502/503/504 is retried like a 429/529, and a zero-value client makes one attempt instead of returning an empty response with no error

## [0.70.2] - 2026-09-20

### Fixed
- fix(sandbox): `bulle remote` passes `-p` only when a port is configured; a bare `user@host` or `~/.ssh/config` alias keeps ssh's own `Port` instead of being overridden with 22, and `remote.port` defaults to 0 so the group no longer forces one
- fix(sandbox): a preflight host-key failure names the host and the command that resolves it — `ssh` for an unknown key (`ssh-keyscan` when the port is literal), `ssh-keygen -R` for a rotated one — instead of only the ssh exit status, and points a loopback target at `bulle sandbox`

## [0.70.1] - 2026-09-20

### Fixed
- fix(sandbox): a boite target starts in the guest's `/workspace` — the directory boite bakes and syncs — not the host directory an instance records; the preflight guard refused every VM session with `project directory "/home/yann" does not exist in target`
- fix(sandbox): a target session with no configured workdir records `target:<name>` as its root, so the banner, system prompt and session header name the target instead of the directory bulle was launched from
- fix(sessions): a `target:<name>` root groups and resumes by target identity, never folded into the launch directory
- fix(usage): usage records for a target session carry the target as the project and no branch, instead of the local repository bulle ran inside

### Changed
- refactor(sandbox): `bulle sandbox list` prints the directory a session's tools run in, rather than the instance's host-side workspace

## [0.70.0] - 2026-09-19

### Added
- feat(sandbox): split sandbox into local boite VMs and bulle remote execution backends

### Fixed
- fix: remove duplicate `session.additional_prompt` definition

## [0.69.0] - 2026-09-18

### Added
- feat(settings): `session.additional_prompt` (`BULLE_ADDITIONAL_PROMPT`, `-additional-prompt`), appended after the base prompt and skills catalog so a specialist persona layers on without replacing the harness guidance
- feat(settings): profiles carry `limits` and `additional_prompt`, keeping identity and model-aware tuning together while tools and security stay in `~/.bulle.yml`

### Fixed
- fix(settings): a profile's identity now beats `~/.bulle.yml` however the profile is selected — before, the file's own `profile:` key lost to the file's other keys while `-profile` and `BULLE_PROFILE` won, so one profile meant two different things and a scaffolded config could name a profile and never see its backend
- fix(docs): `example.bulle.yml` is byte-identical to the first-boot scaffold again, and a test now fails when they drift; the documented `ui.rendering_mode` and `ui.transparent_blocks` defaults follow the code (`tui`, `true`)

## [0.68.0] - 2026-09-18

### Added
- feat(sandbox): host-side agent execution with remote SSH tool remoting for microVM sandboxes
- feat(sandbox): remote implementations of `run_command`, `read_file`, `write_file`, `edit_file`, `list_directory`, `find_files`, and `search_content`
- feat(agent): `RunSessionWithTools` and `RunHeadlessWithTools` for custom tool set mounting

### Changed
- refactor(sandbox): remove guest binary synchronization; guest VMs require zero Bulle binaries and zero API keys
- refactor(sandbox): update preflight check to verify SSH connectivity and workspace readiness without requiring guest binaries

## [0.67.1] - 2026-09-18

### Fixed
- fix(tui): isolate model command unit tests from environment API keys

## [0.67.0] - 2026-09-18

### Added
- feat(tui): `/model` command for dynamic profile and model switching mid-session
- feat(settings): `~/.bulle/profiles/` directory for reusable provider and reasoning configs
- feat(cmd): `--profile` flag and `BULLE_PROFILE` env var for profile selection
- feat(sessions): resume last session by default and generate short 8-char hex session IDs

### Fixed
- fix(tui): preserve slash command autocomplete menu when model picker opens
- fix(provider): add provider catalog inspection and backend construction

## [0.66.0] - 2026-09-17

### Added
- feat(sessions): background session management with `bulle sessions` (alias `bulle list`), attach/resume, and kill
- feat(sessions): detached execution via `--detach` (`-d`) flag, `/detach` TUI command, and `ctrl+d`
- feat(settings): configurable `limits.max_concurrency` and `limits.max_parallel_agents` in `~/.bulle.yml` (default 16)

### Fixed
- fix(parallel): upgrade default subagent concurrency limit to 16 concurrent workers

## [0.65.9] - 2026-09-17

### Added
- feat(sandbox): multi-target sandbox support with ssh and boite backends
- feat(cron): display enabled and crontab installation status in cron list

### Fixed
- fix(editor): use ctrl+g as default keybinding for external prompt editor
- fix(sandbox): direct ssh fallback when boite is not installed

## [0.65.8] - 2026-09-17

### Fixed
- fix(editor): use ctrl+o and alt+e for opening external editor
- fix(sandbox): add list subcommand and styled cobra/fang help for boite VMs
- fix(sandbox): fix guest workdir resolution, shell quoting, and auto binary sync
- fix(cron): improve legacy crontab job line stripping with boundary checks

## [0.65.7] - 2026-09-17

### Fixed
- fix(cron): direct crontab install and uninstall with atomic blocks and log redirection
- fix(cron): automatic PATH augmentation and secret retrieval for unattended runs
- fix(cron): context timeout enforcement and error reporting in delivery logs
- feat(sandbox): isolated boite sandbox environment support

## [0.65.6] - 2026-09-17

### Fixed
- fix(editor): update tests for ctrl+shift+u default and gofmt
- fix: restore sandbox subcommand and suppress linter for unusedfunc
- fix(cmd): remove orphaned newSandboxCmd call
- fix: use markdown for editor prompt file
- chore: rebuild sandbox system per SANDBOX-PLAN.md

## [0.65.5] - 2026-09-16

## [0.65.4] - 2026-09-16

### Fixed
- fix: cron install command installs to crontab
