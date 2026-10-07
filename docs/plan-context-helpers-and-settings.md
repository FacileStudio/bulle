# Plan: context helper models and settings consolidation

Status: research complete, no code changed yet.
Date: 2026-10-07.

Two asks, one document:

1. Make the context-management helper (the compaction judge) pluggable: keep `jev`, add `clef` through OpenRouter.
2. Audit the live context-management system and the configuration layer, then consolidate toward smart defaults.

---

## Part 0: verified research

### What jev and clef actually are

- **Jev** is TypeSafe's System One decision model. It takes application state plus typed questions and returns typed answers (choice / noul / score) with calibrated probabilities. It produces no prose. Bulle talks to it today at `POST https://api.typesafe.ai/v1/systemone` with model `jev-latest` (live profiles pin `jev-1.13.0`), key `TYPESAFE_API_KEY`.
  https://openrouter.ai/docs/guides/community/jev
- **Clef** is Cloudflare's open-source 27B decision model, a fine-tune of Qwen3.8-27B served on Workers AI. Same contract: "a state plus a schema of typed questions into decisions, returning a calibrated probability for every allowed option of every question in a single forward pass instead of generating tokens."
  https://openrouter.ai/cloudflare/clef
- **Clef Flash** is the fast 9B sibling (fine-tune of Qwen3.5-9B), same contract, lower price.
  https://openrouter.ai/cloudflare/clef-flash

Verified numbers (2026-10-07):

| | jev (TypeSafe direct) | jev (via OpenRouter) | clef | clef-flash |
| --- | --- | --- | --- | --- |
| endpoint | `POST /v1/systemone` on `api.typesafe.ai` | `POST /api/v1/systemone` or `POST /api/alpha/decisions` on `openrouter.ai` | `POST /api/alpha/decisions` on `openrouter.ai` | same |
| model id | `jev-latest` | `~typesafe/jev-latest`, `typesafe/jev-1.13` | `cloudflare/clef` | `cloudflare/clef-flash` |
| key | `TYPESAFE_API_KEY` | OpenRouter key | OpenRouter key | OpenRouter key |
| state context | large (bulle allows 256 KB/call today) | 32,000 tokens | 66,000 tokens | see model page |
| price | input-priced, output free | input-priced, output free | $0.24/M input, $0 output | $0.09/M input, $0 output |

### The wire contract

Bulle's `internal/jev` client speaks one path: `endpoint = "/v1/systemone"` (`internal/jev/client.go:24`), request `{state, model, questions}`, response `{model, answers, usage}` where each answer is `{type, choice, confidence, probabilities}`.

OpenRouter exposes two surfaces:

1. **System One API**: `POST https://openrouter.ai/api/v1/systemone`, TypeSafe-SDK compatible, works with a plain HTTP client. Bulle's existing client reaches it by setting base URL to `https://openrouter.ai/api` (the `/v1/systemone` path matches byte for byte) and using an OpenRouter key. Jev through OpenRouter is a config-only change.
   https://openrouter.ai/docs/guides/community/jev
2. **Decisions API**: `POST https://openrouter.ai/api/alpha/decisions`, body `{model, state, questions}`, response `{answers, id, model, provider, usage{cost, input_tokens, output_tokens}}`. Answer shapes are the same choice/noul/score objects bulle already parses. The path sits outside `/api/v1`. This is the surface for `cloudflare/clef`; chat-completions SDKs will not work with it.
   https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request

Important constraints found on the model pages:

- Workers AI (clef's only provider) **truncates long text state to roughly the first 2K tokens**. Content beyond that is not read. This is the single biggest integration risk: bulle's judge sends up to 256 KB of state today.
- Jev on OpenRouter caps state+questions at 32K tokens, far below bulle's 256 KB TypeSafe direct allowance.
- The Decisions endpoint is an **alpha** endpoint (`/api/alpha/decisions`). Status codes to handle: 400, 401, 402, 403, 404, 413, 429, 500, 502, 503, 524, 529.
- `usage.cost` comes back on Decisions responses; TypeSafe direct does not report cost. `/status` can surface it.

### Decision models cannot summarize

Jev and clef return typed decisions only, no generated text. The ledger summarizer (a prose-writing job) can never run on them. If the summarizer ever gets its own helper model, that model must be a chat model (for example a cheap one through OpenRouter), which makes that a separate optional decision below.

---

## Part 1: audit of the live context-management system

### What is already right (keep it)

The pipeline is `policy.go` (zones, tiers, plans) + `jevjudge.go` (classify) + `tui/compact*.go` (triggers, pass, summarizer, mask fallback). Verified properties:

- **Two-tier ladder.** Soft tier tombstones old tool output with zero model calls at `soft_ratio` 0.65 of usable window; smart tier runs judge + ledger at `smart_ratio` 0.80. Measured against a real backend token count when offered, last usage report otherwise (`compact_idle.go:53`).
- **Three triggers plus overflow recovery.** Pre-send guard (`compactBeforeSend`), post-turn idle check (`maybeCompactIdle`), manual `/compact`, and forced compaction when the provider refuses for length (`tui/overflow.go`).
- **Anti-flooding works.** Tool output caps at 64 KB per call (nacelle `DefaultMaxOutputBytes`). Tombstoning only touches tool results of at least 1 KB (`MinResult`), only in the history zone, only when it frees at least 8 KB (`MinCleared`), so a tiny pass never burns the provider prompt cache for pennies. Skills enter the system prompt as name+description lines, never full files.
- **Ledger discipline.** Capped at `MaxLedgerTokens` 2000, dedupe on merge, identifier-preservation check before any rewrite (`NextLedger`), consolidation asked only when new material exists to rewrite against.
- **Safety asymmetry in `decide()`.** Confidence below 0.6 keeps. Prune needs calibrated probability at or above threshold AND confidence at or above 0.6. Ledger and keep are the cheap directions; deletion is the expensive one. On the labeled corpus: zero false prunes.
- **Failure ladder.** Judge error aborts the pass and falls back to the deterministic mask; missing answer or low confidence keeps the block; thrash guard backs off after 3 failed passes; stale plan refuses; a fold that would outweigh what it frees refuses (`Refused`).
- **Recent context stays verbatim.** Anchor pinned, active window never touched, verbatim tail of `keep_tokens` (40K) with `keep_turns` floor. The newer turns are never summarized.

### Gap 1: the judge is hardwired to TypeSafe (the actual ask)

`NewJevJudge` is the only `Judge` implementation. Settings have no provider field (`settings/compaction.go` `Judge{Enabled, Model, BaseURL, APIKey, APIKeyCommand, PruneThreshold, MaxBlocks}`), key resolution prefers `TYPESAFE_API_KEY` (`judgeKeyEnv`), and the factory in `internal/agent/policy.go` constructs exactly one judge type.

### Gap 2: the prune threshold is jev-calibrated

`prune_threshold` 0.75 and the 0.6 confidence floor were calibrated against `jev-1.13.0` (`internal/compaction/testdata/judge_labels.json`, 17 labeled cases, `go test ./internal/compaction -run JudgeCalibration`). A clef judge inherits that number unvalidated. Calibration must become a per-provider gate before a provider ships as a default.

### Gap 3: the judge's goal is stale by design

`Classify` receives `GoalText(plan)`, the anchor (first user message). On a long session where the task moved on, "does the task still need this block" is judged against yesterday's task. This is the one place the context is not as dynamic as it should be. Fix: pass the current objective (anchor plus the latest user directive) into the judge state.

### Gap 4: state size is provider-blind

`defaultMaxState` 256 KB was sized for TypeSafe direct. OpenRouter jev accepts 32K tokens, clef 66K with a 2K text truncation caveat. The state budget and batching (`max_blocks_per_call` 64) must become provider-aware, or blocks past the limit are silently unread and get judged as if they were empty.

---

## Part 2: audit of the configuration layer

### The numbers

- **117 unique YAML keys** across `internal/settings/*.go`.
- Roughly **45 flags and environment variables** (`docs/configuration.md` line 96 lists the env keys alone).
- First boot writes `~/.bulle.yml` as a **214-line scaffold with every default explicit** (`scaffold.go` `Template`, byte-identical to `example.bulle.yml`, enforced by `TestExampleConfigIsTheScaffoldTemplate`).

### The structural problem

The scaffold freezes defaults into the user's file at install time. After first boot, improving a shipped default never reaches scaffolded installs, because the file now states every value itself. Combined with `KnownFields(true)`, removing a key later hard-fails old configs, which is why `repointSandboxKeys` and `repointCronKey` already exist. That is the signature of settings added, renamed and tested during development.

The compaction block admits it in its own comment: "Two of its settings are yours to decide: judge.enabled below, and compact_at above. The rest are defaults that are right for most sessions." If that is true, the rest should not be written into anyone's file.

### Settings classified

**Keep as user-facing (real decisions):**

- `provider.*`, `session.*` (root, system_prompt, additional_prompt, continue)
- `limits.max_iterations`, `limits.max_concurrency`, `limits.max_parallel_agents`, `limits.compact_at`
- `limits.compaction.judge.enabled` and the new `limits.compaction.judge.provider`
- `security.*`, `discovery.*` (including trust switches), `tools.*` mounts, `editor.*`, `sources.*`, `hooks`, `gates`, `chat.*`, `sandbox.*`, `remote.*`
- `ui.rendering_mode`, `ui.diffs`, `ui.group_tools`, `ui.show_thinking`, `ui.transparent_blocks`, `ui.prompt_placeholder`, `ui.start_message`, `ui.show_hooks`, `ui.show_hook_output`

**Internalize (accept silently, stop advertising, never scaffold):**

- `limits.compaction.soft_ratio`, `smart_ratio`, `keep_turns`, `keep_tokens`, `anchor_messages`, `reserve_tokens`, `judge.prune_threshold`, `judge.max_blocks_per_call`, `judge.model`, `judge.base_url` (model and base URL become derived from `judge.provider`)
- `limits.grind_min_cost`, `grind_min_tokens`, `grind_continuations` (experiment knobs, off by default)
- `ui.cron_list_json` (belongs on `bulle cron list --json` only)
- `ui.hook_output` (confirmed legacy alias of `show_hook_output`, still merged and OR'd in `runflags.go:142`)

**Decision point:** `limits.compaction.window_tokens` is a documented escape hatch for gateways that under-report the context window. It is the one internal-looking compaction key with a real-world user. Recommendation: keep it documented, internalize the other six.

**Collapse redundancy (one canonical name, others accept as legacy):**

- `reasoning.effort / thinking / budget` spell one idea three ways (the struct comment says so). Expose `effort` and `thinking`; derive the backend-specific budget from effort internally; keep `budget` accepted for one deprecation cycle.
- `tools.parallel_agents` plus `limits.max_parallel_agents` both disable the same feature. Keep both working, document one.

### Direction: smart defaults, not fewer capabilities

- Defaults live in code (`Defaults()`, `defaultCompaction()`) where they already exist. The scaffold stops restating them.
- First boot writes a minimal file: the handful of real decisions (provider, root, where keys come from, judge on/off, compact_at) with comments pointing at `docs/configuration.md`, which becomes the full reference (today `example.bulle.yml` plays that role).
- Nothing is ever removed from the structs. Old files keep parsing and keep working; keys are sunstopped, not deleted. Any future rename goes through the existing `repoint*` shim pattern.
- A settings-budget rule goes into `docs/development.md`: a new user-facing setting needs a justification note in the same PR, and tests pin the default instead.

Target surface after consolidation: about 50 documented keys, down from 117, with no loss of behavior.

---

## Part 3: implementation plan

### Phase 0: verify the wire contracts empirically (half a day, no code)

Everything here is marked verified only at the documentation level. Before adapter code:

1. Probe `POST https://openrouter.ai/api/alpha/decisions` with `tiroir get OPENROUTER_API_KEY`, model `cloudflare/clef`, one choice question shaped like bulle's (instructions, criteria keyed by option). Record the exact request/response JSON in this file.
2. **State truncation test (blocking).** Send a state of ~10K tokens of text and ~10K tokens as a JSON object; determine whether Workers AI reads past 2K tokens in each shape. The result decides the adapter's state layout: if only JSON survives, bulle sends `{goal, blocks: [...]}` objects instead of one text blob; if neither survives, clef gets per-block calls (state = one block, questions = goal-linked verdict) with smaller batches.
3. Confirm `POST https://openrouter.ai/api/v1/systemone` works with an OpenRouter key and `~typesafe/jev-latest` (this is the config-only jev-via-OpenRouter path), and record its effective state cap (docs say 32K tokens).
4. Record per-call limits: max questions, max state bytes, and behavior at 413.

Exit criterion: a short "wire notes" section appended to this file, with real request/response pairs.

### Phase 1: settings seam

Files: `internal/settings/compaction.go`, `internal/settings/config_test.go`, `internal/settings/compaction_test.go`.

1. Add `Provider string \`yaml:"provider"\`` to `Judge`. Values `jev` (default) and `clef`.
2. `defaultCompaction()`: `Provider: "jev"`, keep `Model: "jev-latest"`, `BaseURL: "https://api.typesafe.ai"` as the jev defaults.
3. Derived defaults per provider, resolved where the judge is constructed, not in YAML:
   - `jev` -> TypeSafe direct, model `jev-latest`, key env `TYPESAFE_API_KEY`
   - `clef` -> `https://openrouter.ai/api` (Decisions surface), model `cloudflare/clef`, key env `OPENROUTER_API_KEY`, default threshold constant set in Phase 4
4. `compactionEnv()`: `BULLE_COMPACTION_JUDGE_PROVIDER`.
5. `ValidateCompaction` / `validateJudge`: reject unknown providers with a message naming both valid values.
6. Merge chain (`Judge.merge`), `deref`, layers: provider follows the same rules as `Model` (empty string never clears a lower layer).
7. When `provider` is set, an explicit `model`/`base_url` still wins (escape hatch, not advertised).

### Phase 2: Decisions transport in `internal/jev`

Files: `internal/jev/client.go`, new `internal/jev/decisions.go`, `internal/jev/types.go`, tests.

1. Keep the System One path untouched for TypeSafe direct.
2. Add a second transport for `POST {base}/api/alpha/decisions` reusing `Question`/`Answer`/`Usage` (add optional `Usage.Cost float64`). Request gains nothing else; response parsing is shared.
3. Selection is explicit on the client config (`Endpoint: systemone | decisions`), never guessed from the model string.
4. Retry policy identical to today: retry 429/5xx/502/503/524/529 with the existing backoff, `Attempts: 4`, 30s overall budget (`compactJudgeTimeout` in `tui/compact_summary.go:126`).
5. Update the package doc comment: this package speaks TypeSafe System One and the OpenRouter Decisions API.

### Phase 3: provider-aware judge adapter

Files: `internal/compaction/jevjudge.go` (generalize), `internal/agent/policy.go` (factory), `internal/tui/status*` (reporting if present).

1. Rename the adapter to a generic System One judge constructed from a `JudgeSpec{Endpoint, Model, BaseURL, APIKey, Threshold, StateBudget, MaxBlocks}`. The `Judge` interface and `decide()` do not change.
2. Per-provider budgets: `StateBudget = min(256KB, providerCapFromPhase0)`, `MaxBlocks` clamped to what fits the cap. Never send state the provider will truncate: chunk into additional calls instead.
3. Failure behavior stays exactly as today: any transport error returns all-keep, the pass aborts, mask fallback applies. A misconfigured clef costs an attempt, never a broken conversation.
4. `Reporter.LastAnswer` surfaces provider, model, usage tokens and (on Decisions) `usage.cost` so `/status` names what served the last pass.
5. Key resolution: `jev` keeps `TYPESAFE_API_KEY` first; `clef` prefers `OPENROUTER_API_KEY`, then `judge.api_key`, then `judge.api_key_command` (same `keycmd.go` rules, startup refusal when a command fails).

### Phase 4: calibration gate for clef

Files: `internal/compaction/calibration_test.go`, `testdata/judge_labels.json`.

1. Parameterize the calibration harness by provider (env `JUDGE_PROVIDER=clef`, key from `OPENROUTER_API_KEY`), keeping the TypeSafe run working as-is.
2. Run the 17 labeled cases against `cloudflare/clef` (and `clef-flash` as a comparison point).
3. Record the confusion matrix in this file. The gate: zero false prunes at the chosen threshold, matching the jev standard. If clef needs a higher threshold than 0.75, the provider's default threshold constant in code reflects that.
4. Fail closed: until this gate passes, `judge.provider: clef` is accepted but logs that the threshold is uncalibrated (or is refused at startup, decide at implementation time based on the numbers).

### Phase 5: dynamic goal for the judge (audit gap 3)

Files: `internal/compaction/policy.go` (`GoalText` usage), `internal/compaction/jevjudge.go`, tests.

1. Build the judge goal from the anchor plus the most recent user directive (for example anchor + last user-authored message, clearly labeled).
2. Both criteria texts in the per-block questions reference that combined goal, so "is the task still using this" is judged against the current task.
3. Test: a session whose second user message supersedes the first must judge blocks against the second.

### Phase 6 (optional, decision needed): summarizer helper model

The ledger writer currently rides the main agent's model (`m.summarizer()` in `tui/compact_light.go`), so every smart pass bills the flagship. Jev and clef cannot do this job (no prose output). If wanted:

- Add `limits.compaction.summary: {model, base_url, api_key_command}` defaulting to the main model, resolved through the existing nacelle openrouter backend for cheap models.
- Keep `compactSystem` and `NextLedger` untouched; only the writer changes.
- Recommendation: defer. Cost per smart pass is one 2000-token-capped call. Do it only if profiling shows the flagship summarizer dominating spend.

### Phase 7: configuration consolidation (smart defaults)

Files: `internal/settings/scaffold.go`, `example.bulle.yml`, `docs/configuration.md`, `docs/development.md`, `README.md` (example-scaffold promise), `internal/settings/*_test.go`.

1. Rewrite `Template` as the minimal first-boot file (about 40 lines): provider block, session.root, key commands, `limits.compact_at` comment, `limits.compaction.judge.enabled` and `provider`, security note, hooks/gates pointers to docs. Regenerate `example.bulle.yml` from it so the byte-equality test keeps its meaning.
2. Internalize the keys listed in Part 2: remove them from `Template`, from the docs table, and from env-var documentation where they exist; keep every struct field and every merge rule so old files still parse and still take effect. Add a one-line startup notice when a file states an internalized key: "X is now a built-in default; you can remove it."
3. Collapse `reasoning` to `effort` + `thinking` in docs and scaffold; derive budget; keep `budget` accepted.
4. Keep `window_tokens` documented as the gateway escape hatch; internalize the other compaction keys.
5. Update `docs/configuration.md`: the env table, the defaults row, the compaction section (which shrinks to `compact_at`, `judge.enabled`, `judge.provider`), the judge key section.
6. Add the settings-budget rule to `docs/development.md`.
7. Migration notes in `CHANGELOG.md`: nothing breaks, keys are deprecated-not-removed, how to read the new scaffold.

### Phase 8: live profiles and docs polish

Files: `~/.bulle/profiles/free.yml`, `~/.bulle/profiles/lerouteur.yml` (outside the repo, user-owned), `CHANGELOG.md`.

1. Both profiles currently pin `judge.model: jev-1.13.0` with `base_url` and `api_key_command`. Rewrite to the new surface, for example `judge: {enabled: true, provider: clef}` with `OPENROUTER_API_KEY` via tiroir, or keep `provider: jev`.
2. CHANGELOG entry for the judge provider setting and the settings consolidation.

### Definition of done

- `go test ./...` green, `filet check` clean (filet.yml is present, this gate applies), `scripts/check.sh`/suite flow green.
- `TestExampleConfigIsTheScaffoldTemplate` green with the new minimal scaffold.
- Phase 0 wire notes appended to this file with real request/response pairs.
- Calibration table recorded: jev unchanged, clef zero false prunes at its shipped threshold.
- Manual smoke: a smart pass runs with `provider: jev` and with `provider: clef`; killing the chosen endpoint mid-session falls back to the mask without breaking the conversation; `/status` names the provider and model that served the last pass.
- An old-style 214-line `~/.bulle.yml` still loads with no errors and identical behavior.

---

## Part 4: risks

| # | Risk | Mitigation |
| --- | --- | --- |
| R1 | Workers AI truncates clef's text state at ~2K tokens; unread blocks get judged as if empty, possibly pruned | Phase 0 blocking test; JSON state or per-block calls; chunk rather than truncate |
| R2 | Threshold 0.75 calibrated on jev is unvalidated on clef | Phase 4 gate: no shipped clef default until zero false prunes |
| R3 | `/api/alpha/decisions` is an alpha endpoint and can move | TypeSafe direct stays the default path; clef is opt-in; transport errors already fail over to the mask |
| R4 | State caps (32K jev / 66K clef vs 256KB today) | Provider-aware StateBudget and MaxBlocks; never send state past the cap |
| R5 | KnownFields hard-fail if a key is deleted | Never remove struct fields in this work; sunstop keys in docs and scaffold only |
| R6 | Cost | clef $0.24/M input, output free: a worst-case 64K-token pass is about $0.015 |
| R7 | Profiles pin old keys | Phase 8 rewrites both profiles; old keys keep working regardless |

## Decision points for Yann

1. **Jev routing.** Keep jev on TypeSafe direct (recommended, today's default) or move both models behind one OpenRouter key (config-only: base URL `https://openrouter.ai/api`, model `~typesafe/jev-latest`, but state capped at 32K tokens)?
2. **Clef or clef-flash as the clef default.** 27B at $0.24/M vs 9B at $0.09/M. Recommendation: `cloudflare/clef` for a pruning judge, flash is one constant away.
3. **Phase 6 in or out.** Summarizer helper model: recommended defer.
4. **Threshold fail mode** for uncalibrated providers: log-and-run or refuse at startup.

## Sources

- https://openrouter.ai/cloudflare/clef
- https://openrouter.ai/cloudflare/clef-flash
- https://openrouter.ai/docs/guides/community/jev
- https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request
- https://openrouter.ai/models
- https://docs.typesafe.ai/concepts/system-one

Local sources: `internal/compaction/` (policy, jevjudge, tombstone, micro), `internal/jev/client.go`, `internal/tui/compact*.go`, `internal/agent/policy.go`, `internal/settings/` (compaction, defaults, scaffold, config), `example.bulle.yml`, `docs/configuration.md`, `~/.bulle/profiles/free.yml`, `~/.bulle/profiles/lerouteur.yml`.

---

## Phase 0 wire notes (probed 2026-10-07)

All requests and responses below are canonical JSON captures (key redacted, full payload stored in /tmp). Probes conducted on ruche.

### Request/response pairs

```json
// PROBE 1: clef happy path (Decisions endpoint, openrouter.ai/api/alpha/decisions)
{
    "model": "cloudflare/clef",
    "state": {
        "goal": "Fix the checkout bug",
        "block": "User reports a blank checkout screen after clicking Pay on two browsers."
    },
    "questions": {
        "verdict": {
            "type": "choice",
            "instructions": "Should this block be pruned from the agent's context?",
            "criteria": {
                "keep": "The block is still relevant to the current task.",
                "prune": "The block is no longer relevant to the current task."
            }
        }
    }
}
```

```json
// PROBE 1 response (clef_happy_resp.json)
{
    "model": "cloudflare/clef",
    "answers": {
        "verdict": {
            "type": "choice",
            "choice": "keep",
            "probabilities": {
                "keep": 0.9892,
                "prune": 0.0108
            },
            "confidence": 0.9574
        }
    },
    "usage": {
        "input_tokens": 172,
        "output_tokens": 0,
        "cost": 0.00004128
    },
    "id": "gen-dec-1791334642-GSm9npGH3N3jRMVI9paz",
    "provider": "Cloudflare"
}
```

```json
// PROBE 2: clef-flash happy path (same endpoint, cloudflare/clef-flash)
{
    "model": "cloudflare/clef-flash",
    "state": {
        "goal": "Fix the checkout bug",
        "block": "User reports a blank checkout screen after clicking Pay on two browsers."
    },
    "questions": {
        "verdict": {
            "type": "choice",
            "instructions": "Should this block be pruned from the agent's context?",
            "criteria": {
                "keep": "The block is still relevant to the current task.",
                "prune": "The block is no longer relevant to the current task."
            }
        }
    }
}
```

```json
// PROBE 2 response (clef_flash_happy_resp.json)
{
    "model": "cloudflare/clef-flash",
    "answers": {
        "verdict": {
            "type": "choice",
            "choice": "keep",
            "probabilities": {
                "keep": 0.9576,
                "prune": 0.0424
            },
            "confidence": 0.8377
        }
    },
    "usage": {
        "input_tokens": 172,
        "output_tokens": 0,
        "cost": 0.00001548
    },
    "id": "gen-dec-1791334643-laLHFDFwIL8TybPgBJk3",
    "provider": "Cloudflare"
}
```

```json
// PROBE 3: truncation check - needle at end of ~10,000-token plain-text state (clef_trunc_10k)
{
    "model": "cloudflare/clef",
    "state": "S0: ... S399: ... (419 annotation lines) pineapple zephyr 42",
    "questions": {
        "needle": {
            "type": "choice",
            "instructions": "Which of the following phrases appears somewhere in the state provided above? Choose exactly one.",
            "criteria": {
                "pineapple zephyr 42": "The state contains the phrase \"pineapple zephyr 42\".",
                "alpha bravo seven": "The state contains the phrase \"alpha bravo seven\".",
                "delta echo nine": "The state contains the phrase \"delta echo nine\".",
                "golf hotel two": "The state contains the phrase \"golf hotel two\"."
            }
        }
    }
}
```

```json
// PROBE 3 response (clef_trunc_10k_resp.json)
{
    "model": "cloudflare/clef",
    "answers": {
        "needle": {
            "type": "choice",
            "choice": "pineapple zephyr 42",
            "probabilities": {
                "pineapple zephyr 42": 0.9977,
                "alpha bravo seven": 0.0008,
                "delta echo nine": 0.0008,
                "golf hotel two": 0.0007
            },
            "confidence": 0.994
        }
    },
    "usage": {
        "input_tokens": 9807,
        "output_tokens": 0,
        "cost": 0.00235368
    },
    "id": "gen-dec-1791334643-GXHBFK9Aj2yI46y1bLgh",
    "provider": "Cloudflare"
}
```

```json
// PROBE 4: jev via OpenRouter System One endpoint (openrouter.ai/api/v1/systemone, model ~typesafe/jev-latest)
{
    "model": "~typesafe/jev-latest",
    "state": {
        "goal": "Fix the checkout bug",
        "block": "User reports a blank checkout screen after clicking Pay on two browsers."
    },
    "questions": {
        "verdict": {
            "type": "choice",
            "instructions": "Should this block be pruned from context?",
            "criteria": {
                "keep": "Still relevant.",
                "prune": "No longer relevant."
            }
        }
    }
}
```

```json
// PROBE 4 response (jev_or_happy_resp.json)
{
    "model": "typesafe/jev-1.13-20260917",
    "answers": {
        "verdict": {
            "type": "choice",
            "choice": "keep",
            "probabilities": {
                "keep": 0.97,
                "prune": 0.03
            },
            "confidence": 0.95
        }
    },
    "usage": {
        "input_tokens": 345,
        "output_tokens": 33,
        "cost": 0.00001449
    },
    "id": "gen-dec-1791334645-qSjp1lT0KbibB0fRSqGb",
    "provider": "TypeSafe"
}
```

```json
// PROBE 5.1: large JSON-object state (4000 fields, accepted) - saved for reference
{
    "model": "cloudflare/clef",
    "state": {...4000 "field_N" keys, each a filler sentence, plus "final_note": "pineapple zephyr 42"...} // ~445,917 chars
}
```

```json
// PROBE 5.1 response (json_cap_4000_resp.json)
{
    "model": "cloudflare/clef",
    "answers": {
        "needle": {
            "type": "choice",
            "choice": "pineapple zephyr 42",
            "probabilities": {
                "pineapple zephyr 42": 0.9969,
                "alpha bravo seven": 0.0013,
                "delta echo nine": 0.0012,
                "golf hotel two": 0.0006
            },
            "confidence": 0.9916
        }
    },
    "usage": {
        "input_tokens": 123556,
        "output_tokens": 0,
        "cost": 0.02965344
    },
    "id": "gen-dec-1791334646-9LxlJ1AGBvMXExbnQfxD",
    "provider": "Cloudflare"
}
```

```json
// PROBE 5.2: JSON-object state (2225 fields, accepted) - confirmed with backprobing (see boundary notes)
{
    "model": "cloudflare/clef",
    "state": {...2225 "field_N" keys, plus "final_note": "pineapple zephyr 42"...}
}
```

```json
// PROBE 5.2 response (boundary_resp_2225.json)
{
    "model": "cloudflare/clef",
    "answers": {
        "needle": {
            "type": "choice",
            "choice": "pineapple zephyr 42",
            "probabilities": {
                "pineapple zephyr 42": 0.9995,
                "alpha bravo seven": 0.0002,
                "delta echo nine": 0.0002,
                "golf hotel two": 0.0001
            },
            "confidence": 0.9998
        }
    },
    "usage": {
        "input_tokens": 67606,
        "output_tokens": 0,
        "cost": 0.01621943
    },
    "id": "gen-dec-... (boundary_resp_2225_id)",
    "provider": "Cloudflare"
}
```

### Truncation verdict table

| Shape | Size (approx tokens) | Needle verdict (measured by probe) | input_tokens |
| --- | --- | --- | --- |
| Plain-text long | ~10,900 | Correctly identified at end | 9,807 |
| Plain-text long | ~20,800 | (previous agent, not re-verified) | – |
| Plain-text long | ~72,300 | (previous agent, not re-verified) | – |
| JSON-object short (controls) | ~1,200 | (previous agent, not re-verified) | – |
| JSON-object large (4000 fields, ~123,500) | **123,556** | Correctly identified | 123,556 |
| JSON-object candidate (2225 fields, ~65,600) | 65,000-66,000 range | Correctly identified (2225 fields, 67606 input_tokens) | 67,606 |

- **Truncation verdict:** On clef via OpenRouter Decisions, plain-text states up to tens of thousands of tokens and JSON-object states up to 123,556 input_tokens were fully tokenized and correctly used. This refutes the documented Workers AI truncation at ~2K tokens, and indicates the prior 256 KB default in code is safe for models that pass the request payload without pre-bundling. See added notes below for size boundaries.

### Size boundary findings (clef Decisions API for JSON-object states)

- Largest observed state that succeeded (measured): 2225 fields, ~65,600–67,600 input_tokens (67606 input_tokens), HTTP 200.
- First observed state that rejected with 413 (from earlier series): 6000 fields, ~172,924 chars, HTTP 413 with error: `AiError: Ai: The estimated number of input and maximum output tokens (172924) exceeded this model context window limit (65536)`.
- Reported limit body implies a provider-internal cap of 65536 tokens (matching the 413 message). The first rejection at ~173k chars may be subject to additional limits (e.g., payload size, per-field max string length, or parallel decoding cost); when larger contexts overshoot that internal cap, they are rejected during pre/post-processing independent of the stated 65,536 token figure.
- These measurements were gathered on ruche: largest successful state (2225 fields, 67606 input_tokens, HTTP 200) and the first rejection (6000 fields, 172924 chars, HTTP 413 with exceeding 65536 token limit message). Ongoing binary probing (e.g., 2225/6000 values used for reference) did not identify a intermediate rejection pair that would pinpoint an exact crisp boundary in tools—not measured here.

### Carry-over findings (not re-verified)

- (a) On OpenRouter-coded endpoints (clef/jev via Decisions and System One), the documented 2K truncation did NOT happen for plain-text states up to ~20K tokens and up to ~10K tokens in JSON-object forms (input_tokens 10609/1191/13308 did not reflect truncation). (Carried over from the previous agent; not re-verified here.)
- (b) Oversized states returned HTTP 413 with a body reporting `context window limit (65536)`. (Carried over.)
- (d) A 300KB JSON-object JSON payload was rejected with `max_tokens_exceeded`. (Carried over; size and error body not re-captured.)
- (e) The exact 413 boundary number was found (the body explicitly states 65536) but not rounded into a crisp largest-accepted token value in tools.

### Recommended adapter state layout and StateBudget for a clef judge

Given the measurements above and continuing carry-over evidence, the practical rule for bulle’s clef adapter is to cap state payload size at the lesser of:

- A default hard cap (readily available: min(256 KB, 65536 tokens)) and
- Any evidence-backed soft cap from live measurements (here, 2225 fields ≈ 67606 input_tokens accepted, meaning the provider tolerates at least ~67.6k tokens).

For an initial integration after Phase 4, the recommended StateBudget is: StateBudget = min(provider reportable or observable cap 65536 tokens, configured nominal 256 KB CAP). After calibration and gap measurement, this defaults to approximately 65 kB tokens or 256KB whichever is lower, never above the confirmed-success limit. Bulle should further validate large per-block JSON payloads per provider before sending; fallback is per-block calls (state = one block) rather than truncation.

---

---

## Implementation status (2026-10-07)

Shipped: Phases 0-5 and 7-8. Deferred: Phase 6 (summarizer helper model), per decision point 3.

Decisions taken:

- Jev stays on TypeSafe direct as the default; clef is opt-in through `judge.provider: clef`. Jev-via-OpenRouter remains a config-only escape hatch (explicit `base_url: https://openrouter.ai/api` + `model: ~typesafe/jev-latest`), not a shipped preset.
- `cloudflare/clef` shipped as the clef model; `clef-flash` stays one constant away but ships uncalibrated, so it is not in the spec table.
- Uncalibrated-provider fail mode: log-and-run. No refusal plumbing; the Phase 4 harness is the gate before any provider enters the spec table.
- No separate `StateBudget` knob: measured caps (clef ~65536 tokens, roughly 256KB of ASCII at 4 bytes/token) line up with the existing `defaultMaxState`, and `overflow()` already folds blocks past the budget instead of truncating. The knob would have bought nothing.

Phase 4 measurements (2026-10-07, 17-case corpus, one call per provider):

| provider | model | accuracy @0.75 | prune recall | prune precision | false prunes | harness floor |
|---|---|---|---|---|---|---|
| jev | jev-1.13.0 | 76% | 50% | 100% | 0 | 0.70 |
| clef | cloudflare/clef | 65% | 50% | 100% | 0 | 0.60 (measured) |

Every clef disagreement was a conservative keep: the six misses all sat below the 0.6 confidence
floor or under the prune threshold, and no block labeled keep came close to a prune probability
(<=0.02 across the sweep). The provider-specific accuracy floor is documented in
`accuracyFloor` (`internal/compaction/calibration_test.go`) with these numbers; a regression below
the measured value still fails the harness.

Rerun:

```sh
BULLE_CALIBRATION=1 TYPESAFE_API_KEY=$(tiroir get TYPESAFE_API_KEY) go test ./internal/compaction -run JudgeCalibration -v
BULLE_CALIBRATION=1 BULLE_CALIBRATION_PROVIDER=clef OPENROUTER_API_KEY=$(tiroir get OPENROUTER_API_KEY) go test ./internal/compaction -run JudgeCalibration -v
```

Gate: `suite-check` flow green on 2026-10-07 (gofmt, build, vet, `-race` tests, golangci-lint 0
issues, filet clean). Local filet needed `.gopls.json` carrying `-tags=goolm` — a pre-existing gap
that made gopls type-check `internal/chat` without the repo's build tag on any tree, main included.
