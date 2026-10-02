# bulle — Configuration

Every field the core `Config` reads, then the client's own settings layer — a separate thing,
with its own precedence order — and the traps in each.

## `bulle.Config`

The core reads nothing from the environment or from disk. Every field is passed in the struct
literal a consumer builds.

| Field | Default | What it does |
|---|---|---|
| `Backend` | — required | The model this agent runs on. No default: a package that picks one for you is a package that hides the most consequential decision in the configuration |
| `System` | — required | The system prompt. Empty is refused rather than defaulted — an agent with no system prompt is a general-purpose assistant wearing a product's name |
| `Thinking.Effort` | the backend's own default | `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`. `none` is the only value that asks for no reasoning, and a model whose reasoning is mandatory answers it with a 400 rather than ignoring it (measured on `stealth/ox-alpha`, 2026-08-23). Empty hands the decision to the provider, which on a model whose reasoning is mandatory means that model's own maximum. Not validated against the model: OpenRouter clamps a level a model does not advertise rather than refusing it |
| `Thinking.Budget` | `0` (no ceiling from here) | Reasoning-token ceiling for one turn. The other spelling of `Effort`. Anthropic takes both at once; OpenRouter answers the pair with a 400 (`Only one of reasoning.effort and reasoning.max_tokens can be specified`, measured 2026-08-23) and its backend resolves that by letting the budget win, since the gateway defines each level as a percentage of the budget anyway. Refused at construction when it cannot work: at or above `MaxTokens` it leaves the turn nothing to answer with, and below a backend's `Capabilities.MinBudget` (1024 on `anthropic`) the API rejects it |
| `Thinking.Show` | `false` | Stream the model's reasoning as `KindThinking`. Off by default, matching the APIs: the raw chain of thought is never returned and a readable summary is opt-in. It changes what a consumer is shown and nothing else. The reasoning always travels back over the wire, because that is what the tool loop replays to keep the model's train of thought intact across a tool call |
| `MaxTokens` | `DefaultMaxTokens` (32000) | Per-turn output ceiling. Generous on purpose — every request is streamed, so a large ceiling costs nothing in latency, while a small one truncates an answer mid-sentence and buys a retry |
| `MaxIterations` | `0` (no cap) | How many times the model may be asked. A cap is only safe when every tool is read-only and cheap; reaching it ends the run with `Stop: StopIterations`, not a failure |
| `Tools` | `nil` | Tools the model may call in this process |
| `MCP` | `nil` | MCP servers the model may call tools on. Only `anthropic` can reach them — see [architecture.md](architecture.md#capabilities) |
| `Logger` | `slog.Default()` | Receives the few things worth recording that are not events — retry attempts, mainly |

`Backend`, `Thinking` and `MCP` are all checked against the chosen backend's
`Capabilities` at construction; a config asking for something the backend cannot do is refused
with an `*Unsupported` error rather than silently running with less.

## `anthropic.Config`

| Field | Default | What it does |
|---|---|---|
| `Client` | built from the environment | Set to share a client, point at a proxy, or hand a test a stub transport |
| `Model` | `DefaultModel` (`claude-opus-5`) | — |

## `openrouter.Config`

| Field | Default | What it does |
|---|---|---|
| `APIKey` | `OPENROUTER_API_KEY` | — |
| `Model` | — required | An OpenRouter model slug, e.g. `anthropic/claude-opus-5`. Required: OpenRouter fronts hundreds of models and a default would be this package choosing the one thing the caller came here to choose |
| `BaseURL` | `DefaultBaseURL` | Point at a compatible gateway or a recording proxy in tests |
| `Referer`, `Title` | — | Attribution only — `HTTP-Referer` and `X-OpenRouter-Title`. Neither affects the answer; without `Referer` the usage simply does not appear against an app |
| `Provider` | `nil` | OpenRouter's provider-routing object, passed through untouched. Worth setting `require_parameters: true` when tool calling matters — it keeps the request away from providers that would drop the tool schema |
| `Options` | `nil` | Extra request options for the underlying client |

## `google.Config`

| Field | Default | What it does |
|---|---|---|
| `APIKey` | `GEMINI_API_KEY` or `GOOGLE_API_KEY` | — |
| `Model` | `DefaultModel` (`gemini-3.7-flash`) | The Gemini model to run |
| `BaseURL` | `DefaultBaseURL` (`https://generativelanguage.googleapis.com/v1beta/openai`) | Point at a compatible proxy or gateway |
| `Options` | `nil` | Extra request options for the underlying client |

## `openai.Config`

| Field | Default | What it does |
|---|---|---|
| `APIKey` | `OPENAI_API_KEY` | — |
| `Model` | `DefaultModel` (`gpt-5.4`) | The OpenAI model to run |
| `BaseURL` | `DefaultBaseURL` (`https://api.openai.com/v1`) | Point at a compatible gateway or proxy in tests |
| `Organization` | `""` | Optional `OpenAI-Organization` header |
| `Project` | `""` | Optional `OpenAI-Project` header |
| `Options` | `nil` | Extra request options for the underlying client |

## `tools.Config`

| Field | Default | What it does |
|---|---|---|
| `Root` | — required | The directory every tool is confined to, through `os.Root` |
| `AllowBash` | `false` | Mounts the command tool. Not a limit that can be tuned into safety — running commands is either something this agent may do or it is not |
| `CommandEnv` | `PATH` + `HOME` only | The process environment is never inherited — it is where a service keeps the credentials a model must not read |
| `CommandTimeout` | `DefaultCommandTimeout` (2 min) | A command outliving it is killed with its children |
| `MaxOutputBytes` | `DefaultMaxOutputBytes` (64 KiB) | Caps what one tool call may return — output stays in the context window for the rest of the conversation |
| `MaxReadBytes` | `DefaultMaxReadBytes` (48 KiB) | Caps a single file read, below the output cap so a read that hit the limit still leaves room for the notice saying so |

## The TUI's settings layer

The client's settings layer is a separate settings problem from the core, and deliberately does not share a
mechanism with it: **the library must never read configuration from disk or environment.** A
`nacelle` package that read `~/.bulle.yml` would let a file on a *different* machine's disk
change a headless consumer's behaviour, the way Bubble Tea's own config never reaches past the
program that imports it.

### Precedence

**Flag beats environment beats file beats default**, resolved in one function
(`settings.Load()` in `internal/settings/settings.go`) and nowhere else. The suite has already paid for the
alternative once: a CLI that read its environment inside one code branch and its file inside
another turned what its README called "overrides" into two mutually exclusive modes nobody
could tell apart.

| Layer | Source | Notes |
|---|---|---|
| Flags | `-backend`, `-model`, `-effort`, `-root`, `-system-prompt`, `-additional-prompt`, `-bash`, `-thinking`, `-project-context`, `-skills`, `-trust-skills`, `-skill-dir`, `-mcp`, `-fetch`, `-approve-tools`, `-diffs`, `-show-hooks`, `-show-hook-output`, `-max-iterations`, `-compact-at`, `-max-concurrency`, `-max-parallel-agents`, `-tasks`, `-continue`, `-resume`, `-gates-file`, `-no-config` | Only flags actually **typed** are collected, via `flag.Visit` — Go's `flag` package cannot otherwise tell a flag left alone from one passed its own default value. `-skill-dir` and `-mcp` are repeatable (`-mcp a.json -mcp b.json`); every other flag keeps only its last occurrence. `-resume` names one session by id or file path and, when given, beats `-continue`. `-no-config` skips `~/.bulle.yml` entirely: defaults plus environment plus flags. An invalid file gets a coloured report and one prompt — yes boots with defaults, no exits with the documentation link |
| Environment | `BULLE_BACKEND`, `BULLE_MODEL`, `BULLE_PROVIDER_BASE_URL`, `BULLE_PROVIDER_API_KEY`, `BULLE_EFFORT`, `BULLE_REASONING_BUDGET`, `BULLE_ROOT`, `BULLE_SYSTEM_PROMPT`, `BULLE_ADDITIONAL_PROMPT`, `BULLE_BASH`, `BULLE_THINKING`, `BULLE_PROJECT_CONTEXT`, `BULLE_SKILLS`, `BULLE_TRUST_SKILLS`, `BULLE_SKILL_DIRS`, `BULLE_APPROVE_TOOLS`, `BULLE_DIFFS`, `BULLE_SHOW_HOOKS`, `BULLE_SHOW_HOOK_OUTPUT`, `BULLE_MAX_ITERATIONS`, `BULLE_COMPACT_AT`, `BULLE_MAX_CONCURRENCY`, `BULLE_MAX_PARALLEL_AGENTS`, `BULLE_COMPACTION_SOFT_RATIO`, `BULLE_COMPACTION_SMART_RATIO`, `BULLE_COMPACTION_WINDOW_TOKENS`, `BULLE_COMPACTION_RESERVE_TOKENS`, `BULLE_COMPACTION_KEEP_TURNS`, `BULLE_COMPACTION_KEEP_TOKENS`, `BULLE_COMPACTION_ANCHOR_MESSAGES`, `BULLE_COMPACTION_JUDGE`, `BULLE_COMPACTION_JUDGE_MODEL`, `BULLE_COMPACTION_JUDGE_BASE_URL`, `BULLE_COMPACTION_JUDGE_API_KEY`, `BULLE_COMPACTION_PRUNE_THRESHOLD`, `BULLE_COMPACTION_MAX_BLOCKS`, `BULLE_FETCH`, `BULLE_TASKS`, `TYPESAFE_API_KEY` | A misspelt boolean (`BULLE_BASH=yez`) is treated as unmentioned, not as `false`, and falls through to the layer below. `BULLE_SKILL_DIRS` is colon-separated, the same convention `PATH` itself uses for a list of directories. `BULLE_PROVIDER_BASE_URL` and `BULLE_PROVIDER_API_KEY` belong to the active provider — see [Custom providers](#custom-providers). `TYPESAFE_API_KEY` is the compaction judge's key and beats `BULLE_COMPACTION_JUDGE_API_KEY`; it is the one credential to keep in the environment rather than the file |
| File | `~/.bulle.yml` | Preferences only, **no credentials** — those already have two homes: the environment, and the Anthropic SDK's own profile. A file can avoid holding one even where it needs one: `api_key_command` names the program that prints the key (`provider.api_key_command`, `limits.compaction.judge.api_key_command`) — see [Keeping the key out of the file](#keeping-the-key-out-of-the-file). `KnownFields(true)`: an unrecognised key (`max_iteration:`, one letter short) is refused rather than silently ignored |
| Defaults | — | `provider.backend: anthropic`, `root: .`, `tools.run_command: true`, `reasoning.thinking: true`, `discovery.project_context: true`, `discovery.skills: true`, `discovery.trust_skills: false`, `discovery.trust_hooks: false`, `sources.skill_dirs: []`, `sources.mcp: {}`, `security.approve_tools: false`, `security.deny_elevation: true`, `ui.diffs: true`, `ui.show_hooks: true`, `ui.show_hook_output: true`, `limits.max_iterations: 5`, `limits.compact_at: unset` (an absolute override when set; unset derives the ceiling from `soft_ratio` × the context window, `0` disables), `limits.max_concurrency: 16`, `limits.max_parallel_agents: 16`, `limits.compaction.soft_ratio: 0.65`, `limits.compaction.smart_ratio: 0.80`, `limits.compaction.keep_turns: 1`, `limits.compaction.keep_tokens: 40000`, `limits.compaction.anchor_messages: 1`, `limits.compaction.judge.enabled: false`, (`limits.compaction.window_tokens` and `reserve_tokens` are unset: the first is the backend's own window, the second a fifth of it), `tools.web_fetch: true`, `tools.tasks: true`, `tools.parallel_agents: true`, `ui.rendering_mode: tui`, `ui.group_tools: true`, `ui.show_thinking: true` |

`project_context` and `skills` default **on**, unlike `bash`: each fails soft to nothing when
there is nothing to find — no `AGENTS.md`/`CLAUDE.md` anywhere above `root`, no
`~/.agents/skills/` — so a machine without either is no worse off for asking. `trust_skills` and `approve_tools` both default **off**, and for the same
underlying reason: a project's own `.agents/skills/` can carry instructions to run arbitrary
scripts, and asking before every tool call is a decision that changes how the client feels to
use — blanket-trusting a directory, or blanket-interrupting every call, are both decisions only
the person running this should opt into, not defaults sprung on them. See
[Context and skills](#context-and-skills) and
[Tool approval](#tool-approval) below.

### `~/.bulle.yml`

The file is grouped: each family of settings lives under its own key, the same
families the code groups them into. The old flat layout (every key at the top
level) was replaced by these groups in 0.44.0 — a flat key is now a parse
error, because `KnownFields(true)` refuses anything unknown.

```yaml
provider:
  backend: anthropic
  model: claude-opus-5
  # base_url:, api_key: and api_key_command: point at a custom endpoint and keep
  # the secret out of the file — see "Custom providers" and "Keeping the key out
  # of the file". Leave them out to use the vendor's own API and key resolution.
  # base_url: http://localhost:3001/v1
  # api_key: sk-your-unified-key
  # api_key_command: tiroir get ANTHROPIC_API_KEY
reasoning:
  effort: high
  thinking: true
  budget: 8192
limits:
  # compact_at is an absolute token ceiling. Left unset (the default) the
  # ceiling comes from the context window and soft_ratio below; 0 turns
  # compaction off outright.
  # compact_at: 75000
  max_iterations: 5
  max_concurrency: 16
  max_parallel_agents: 16
  # compaction decides, per history block, what to keep, prune or fold into the
  # state ledger. Two of its settings are yours to decide: judge.enabled below,
  # and compact_at above. The rest are defaults that are right for most sessions.
  #
  # The ratios are fractions of the window a turn can fill: the backend's window
  # less reserve_tokens, the runway held back for the model's own answer.
  # soft_ratio is the free rung; smart_ratio is the paid one. Raise soft_ratio
  # before touching smart_ratio.
  compaction:
    soft_ratio: 0.65
    smart_ratio: 0.80
    # window_tokens overrides what the backend reports — gateways under-report
    # and some runners report nothing at all. reserve_tokens left out is a fifth
    # of the window, between 8k and 64k.
    # window_tokens: 200000
    # reserve_tokens: 40000
    # keep_turns is a floor in messages the verbatim tail never drops below, and
    # keep_tokens is the budget that sizes it.
    keep_turns: 1
    keep_tokens: 40000
    anchor_messages: 1
    # The judge is opt-in and off by default: enabling it sends conversation
    # history to TypeSafe (see "Compaction" below).
    judge:
      enabled: false
      model: jev-latest
      base_url: https://api.typesafe.ai
      api_key: ""
      # api_key_command: tiroir get TYPESAFE_API_KEY
      prune_threshold: 0.75
      max_blocks_per_call: 64
session:
  root: .
  system_prompt: You are a terminal coding assistant.
  # appended after the base prompt and everything layered onto it; leave it out
  # to keep the harness prompt alone, or set it to layer a persona on top.
  additional_prompt: Keep answers under 200 words; always name the file you changed.
  continue: false
tools:
  run_command: true
  web_fetch: true
  tasks: true
  parallel_agents: true

security:
  approve_tools: false
  path_isolation: false
  deny_elevation: true
  env_isolation: false

discovery:
  project_context: true
  skills: true
  trust_skills: false
  trust_hooks: false
ui:
  prompt_placeholder: 'Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\ forces it.'
  rendering_mode: tui
  group_tools: true
  show_thinking: true
  diffs: true
  transparent_blocks: true
  show_hooks: true
  show_hook_output: true
  # cron_list_json: false
  # start_message: |-
  #   Welcome to bulle.
  #     _   _
  #    | | | |
  #    | |_| |
  #    |_____|
sources:
  skill_dirs:
    - ~/.claude/skills
  mcp:
    mycelium:
      command: mycelium
      args: [mcp]
hooks: []
# Gates are deterministic checks run after every edit: the edited path is
# appended to the command's arguments, findings are injected into the edit
# result. No gates key keeps the built-in filet check.
gates: []
# gates:
#   - name: filet
#     command: [filet, check]
#     timeout_secs: 30
#   - name: prettier
#     command: [prettier, --write]
#     format: true
```

Old flat key → new home, for migrating a pre-0.44 file:

| Old key | New location |
|---|---|
| `backend`, `model`, `base_url`, `api_key` | `provider:` |
| `max_iterations`, `compact_at`, `max_concurrency`, `max_parallel_agents` | `limits:` |
| `bash` | `tools.run_command` |
| `subagents` | `tools.parallel_agents` |
| `fetch` | `tools.web_fetch` |
| `tasks` | `tools.tasks` |
| `approve_tools`, `path_isolation`, `deny_elevation`, `env_isolation` | `security:` |
| `effort`, `thinking`, `reasoning_budget` (now `budget`) | `reasoning:` |
| `project_context`, `skills`, `trust_skills`, `trust_hooks` | `discovery:` |
| `continue` | `session:` |
| `resume` | dropped from the file — `-resume` on the command line only |
| `mode` (now `rendering_mode`), `json` (now `cron_list_json`), `group_tools`, `show_thinking`, `diffs`, `prompt_placeholder`, `start_message`, `transparent_blocks` | `ui:` |
| `skill_dirs`, `mcp` | `sources:` |

Every field is optional. A missing file is not an error — on first boot
bulle writes this file itself with every default explicit, and you edit from
there; deleting it regenerates it. But an unreadable or malformed one is: a
unreadable or malformed one is: a config silently ignored is worse than no config, because the
setting carefully written is simply not in effect and nothing says so.

Scheduled jobs are no longer part of this file at all — they live one YAML file
per job in `~/.bulle/jobs/`. See [Cron jobs](#cron-jobs) for the folder, its
trust gate and the per-job overrides.

`prompt_placeholder` is the ghost text shown while the prompt is empty. The prompt has no prefix:
the first row opens at a single margin space, and wrapped rows of a long question hang under it.

`start_message` is printed as the first thing on launch, above the version banner. It is a plain
string that may span lines, so use a `|`-literal (or `|-` to drop the trailing newline) to put a
welcome block or an ascii banner there. Empty (the default) prints nothing.

**No per-project `./.bulle.yml` yet.** A second precedence layer before the first has real
users is a layer nobody has asked for the shape of.

### Custom providers

A custom provider points bulle at an endpoint that is not one of the four vendor APIs, using an
existing backend's protocol when the endpoint speaks it. It is four fields with the same precedence
as everything else: `backend` and `model` (the `BULLE_BACKEND` / `BULLE_MODEL` behind them), plus
`base_url` and `api_key` (the new ones), all read into one `Provider` group.

The common case is an OpenAI-compatible gateway — any server that speaks the
`v1/chat/completions` dialect the `openai` backend does, a class that includes most self-hosted
proxies. Point `backend` at `openai`, set your own `base_url` and `api_key`, and the gateway is used
as-is:

```sh
export BULLE_BACKEND=openai
export BULLE_PROVIDER_BASE_URL=http://localhost:3001/v1
export BULLE_PROVIDER_API_KEY=sk-your-unified-key
export BULLE_MODEL=auto
bulle
```

Or the same provider in `~/.bulle.yml`:

```yaml
provider:
  backend: openai
  base_url: http://localhost:3001/v1
  api_key: sk-your-unified-key
  model: auto
```

`base_url` and `api_key` are optional everywhere. Leave `base_url` out and the backend's own default
stands (`https://api.openai.com/v1` for `openai`); leave `api_key` out and the backend falls back to
its usual environment variable (`OPENAI_API_KEY` and the rest). The `openai` backend always requires
*a* key, so for a genuinely keyless local endpoint set `OPENAI_API_KEY` to any non-empty dummy value;
the config errors otherwise. They only matter together with a custom endpoint.

The file note from the precedence table above softens here: the config file **can** hold an
`api_key`, for a private endpoint where a committed dotfile is the reasonable home for it. The old
warning still stands for vendor keys — a file holding an `OPENAI_API_KEY` you actually use is a file
that can never be committed to a dotfiles repo. A custom endpoint like freellmapi has its own key,
so the two stories do not collide. In either case the env-var route is the one that keeps the key
out of any file, and `BULLE_PROVIDER_API_KEY` shares the `BULLE_` prefix the rest of the layer
uses.

`anthropic` cannot be pointed at a custom endpoint: its `Config` takes a pre-built client rather
than a URL and key. `openai`, `openrouter` and `google` all take a `BaseURL`. A local OpenAI-compatible
gateway is the overwhelmingly common case, and it is the case that works.

### Keeping the key out of the file

`api_key_command` replaces the literal key with the command that produces it, so a config file — or a
profile, or a dotfiles repo — carries no secret at all:

```yaml
provider:
  backend: openrouter
  model: deepseek/deepseek-v4.1-flash
  api_key_command: tiroir get OPENROUTER_API_KEY

limits:
  compaction:
    judge:
      enabled: true
      api_key_command: tiroir get TYPESAFE_API_KEY
```

The provider's key is `provider.api_key_command` and the judge's is
`limits.compaction.judge.api_key_command`. Either may sit in a profile, which is where the setting
earns its keep: one file per identity, none of them holding a credential.

The command runs through `sh -c`, so a whole program with its arguments, a pipeline, or
`tiroir unlock && tiroir get K` all work. It prints the key on stdout; trailing whitespace is
trimmed. stderr is read for the error message and never for the key, so a command that warns on
stderr still works. Its stdin is closed rather than inherited, so a command that stops to prompt
fails against end-of-file instead of seizing your terminal, and it is killed after 10 seconds.

**It is a source, not an override.** A key any layer already supplied wins — a flag, the
environment, a literal `api_key`, a profile — and the command only fills what is still empty. The
practical consequence is worth stating plainly: **if `TYPESAFE_API_KEY` is exported on your machine,
a judge `api_key_command` never runs**, because the environment already answered. Same for
`BULLE_PROVIDER_API_KEY` or an exported `OPENROUTER_API_KEY` against a provider command. Unset the
variable when you want the command to take over. Nothing else changes: a key that arrives from the
environment keeps working with no config edit at all.

Two more properties, both deliberate:

- The command runs only where a key is actually used — when a session builds its provider or the
  judge — and never for inspection. `bulle list`, `bulle sessions` and `bulle cron list` do not run it,
  so a locked secret store cannot break them.
- A command that fails, times out or prints nothing is **refused at startup**, naming the field,
  rather than left as an empty key. An empty key would reach the backend as "no credential" and read
  exactly like a config that forgot one. A locked tiroir fails loudly instead.

The environment-variable route remains the shortest one when the key is already exported; the
command route is for when it is not, and for the files you keep in git.

### Profiles and /model command

Profiles define reusable identities in `~/.bulle/profiles/<name>.yml`. Bulle reads any `.yml` file in that folder on startup and during interactive sessions.

A profile carries **identity and model-aware settings**: which backend and model to reach, how
hard that model should think, the limits that suit it, and the persona layered on top of the
prompt. **Behaviour** — tools, security, sources, gates — stays in `~/.bulle.yml`. That split is
the point: one profile then works in every checkout without carrying that checkout's environment
along, and a single edit to `~/.bulle.yml` moves every profile at once.

```yaml
# ~/.bulle/profiles/fast.yml
name: fast
provider:
  backend: google
  model: gemini-2.5-flash
reasoning:
  effort: low
  thinking: true
  budget: 2048
# limits tune a run to the model that is running it: a fast model can take more
# turns, a large one may want to compact later.
limits:
  max_iterations: 8
  compact_at: 100000
# the persona, appended after the base prompt exactly like session.additional_prompt
additional_prompt: Answer in short paragraphs and name every file you change.
```

You can select a profile in `~/.bulle.yml` using the top-level `profile` key:

```yaml
profile: fast
```

Or pass `-profile <name>` on the command line, or set `BULLE_PROFILE=<name>` in the environment. Whichever layer names a profile, it applies in the same place. Precedence order follows:

```
defaults < ~/.bulle.yml < profile < environment < flags
```

A profile is a layer of its own, and it sits **above `~/.bulle.yml`**: the identity it names wins
over the file, so `profile: fast` moves the backend even when the file names one too, and one
profile then works in every checkout. It is not a complete override — the environment and the
flags still win over it, field by field, so a `BULLE_MAX_ITERATIONS` or `-max-iterations` beats the
limit a profile sets, and what is specific to *this* machine survives a profile the whole suite
shares. Exactly one `additional_prompt` reaches the prompt: a profile's persona replaces the
file's rather than stacking with it. And a profile's limits and persona take effect at launch
only: a mid-session `/model` swap changes the provider and the reasoning, and nothing else. The
system prompt and the run's limits stay as they were when the session started.

During an interactive session, the `/model` command switches models on the fly:

- `/model` opens an interactive dropdown menu listing discovered profiles and detected provider models.
- `/model <name>` switches immediately to the named profile or model string (e.g. `/model fast` or `/model openai/gpt-5.4`).

The conversation history remains intact when switching models mid-session.

### Compaction

Compaction used to trip at one absolute token count. It now has a ladder: the session measures
itself against the backend's context window and acts at the ratio it crossed, before it reaches
any ceiling at all.

| Tier | Crossed at | What it does | Model calls |
|---|---|---|---|
| soft | `soft_ratio` × usable window | Tombstones oversized history tool results older than the active window — deterministic, no model call | 0 |
| smart | `smart_ratio` × usable window | Classifies each history block and keeps, prunes or folds it into one `[state ledger]` message, forcing the fold when the gentle one would not land | 1 judge + 1 ledger |

Whether a pass **forces** — folding the whole history rather than the blocks the judge left in
place — is not a third threshold. It is derived: the pass forces when the gentle fold it just
built would not leave the conversation under its trigger. Asking that directly is more precise
than a second ratio, which had to serve every window size at once, and it is safe because
forcing only ever moves a block from kept to folded. A `prune` still needs the judge's
probability and confidence, so forcing costs verbatim fidelity and never a fact.

The **usable window** is the backend's context window less `reserve_tokens`, the
runway held back for the model's own answer. Measuring the ladder against it is
what stops the top rung from leaving a reasoning model nothing to think in: against
the raw window a ratio at 0.90 would leave only a tenth of it for the response,
where against the usable window the top rung leaves the reserve *and* a fifth of
what a turn can fill. The reserve defaults to a fifth of the window within the
shipped bounds (8k–64k); set `reserve_tokens` when you know what the model needs,
and `window_tokens` when the backend under-reports a window or reports none at
all — an OpenAI-compatible runner reports zero, which otherwise leaves every
session on one global `75000`.

Only tool results are tombstoned, not reasoning: every backend drops a `Reasoning` block when it
builds a request (Anthropic wants the signature it was issued with, and the stream never carries
one), so a stub on it would free no context while editing a transcript you can still scroll back
to. What the soft tier reclaims is the tool output. A ratio outside `(0,1]`, or a ladder whose
rungs are out of order, is refused at load rather than quietly never firing — `soft_ratio: 0` would
derive a zero ceiling, which every trigger reads as "compaction off".

`limits.compact_at` still speaks last: a value someone set is an absolute ceiling and beats the
ratios, `0` turns compaction off, and leaving it unset is what puts the ladder in charge. A
backend that reports no context window cannot use a ratio at all and falls back to `75000`, the
same number `DefaultCompactAt` has always carried. `keep_tokens` sizes the verbatim tail as a
token budget and `anchor_messages` pins the head: two heavy file reads can no longer hold the
window open by virtue of being the newest turns, and the first user turn is never rewritten,
summarized or pruned, so the original task cannot be summarized away. `keep_turns` is the floor
under the budget — how few messages the tail may ever shrink to, one by default, the live turn
alone, which is also the one turn no summary may stand in for. Neither bound may take more than
half of what a pass may fill — a `keep_turns` whose own turns would is refused at load, whenever a
`window_tokens` or a `compact_at` is there to bound the cap — so a tail can never defeat the pass it triggers. With the judge off (the default), a pass folds the whole history into the ledger. With it on, a pass classifies each history block and keeps, prunes or folds it — and folds the rest too when the keeps would not land the conversation under its trigger.

The ledger is only ever *merged* into: a later pass folds new facts in line by line, and a
summarizer that restates what the ledger already holds adds nothing to it. It is never handed to a
summarizer alone — every call that rewrites it also carries turns no earlier pass compressed — and
once the body outgrows one summary (2000 tokens, the ceiling the summarizer itself writes under) the
pass *consolidates* it instead: one rewritten block, accepted only if it still names every
identifier the old body named, and otherwise the merge stands. If a provider refuses a request for
length anyway, bulle compacts once and sends the turn again — one forced pass, at most once per
turn, and not at all while `compact_at` is `0`.

**The judge is opt-in and off by default.** Turning on `limits.compaction.judge` sends
conversation history — which can include source code and secrets — to TypeSafe's System One model
for classification, so it is never a shipped default and has to be asked for. It is the only
setting here that leaves the machine. Its key prefers the `TYPESAFE_API_KEY` environment variable
over `limits.compaction.judge.api_key`. With the judge off, compaction behaves exactly as it did
before this ladder existed.

Enabling it is one key in `~/.bulle.yml`. No flag and no environment variable are required:

```yaml
limits:
  compaction:
    judge:
      enabled: true          # sends history to TypeSafe on every summarizing pass
      model: jev-latest
      base_url: https://api.typesafe.ai
      api_key: ""            # empty is fine: export TYPESAFE_API_KEY instead
      prune_threshold: 0.75  # a block is pruned at this probability and over
      max_blocks_per_call: 64
```

A missing key is not a startup error, because `base_url` may point at a proxy that wants none. The
first pass is what reports it: it fails with `typesafe: unauthorized (401)`, prunes nothing, and
falls back to the deterministic mask, so a misconfigured judge costs an attempt and never a broken
conversation.

`/status` names the model version that actually answered the last call, because `model` defaults to
the vendor's drifting `jev-latest` alias and the thresholds were tuned against one build behind it.
Once the ladder behaves the way you want, pin `model` to that versioned id so the tuning survives
the next release.

### Tuning the judge thresholds

`prune_threshold` and the confidence floor are the two numbers that decide whether a block is
deleted, and both are shipped defaults rather than measured ones. The calibration harness is how you
check them against the model: it classifies a corpus of labeled history blocks in one call, re-scores
the answers across a sweep of thresholds, and prints where the model's confidence and its accuracy
part company.

```sh
TYPESAFE_API_KEY=... go test ./internal/compaction -run JudgeCalibration -v
```

It skips without a key, so it stays out of CI. The corpus is
`internal/compaction/testdata/judge_labels.json` — a seed of seventeen cases with the verdict a careful
operator would give each one. Extend it with blocks from your own traces before trusting the numbers;
the harness fails when a block labeled `keep` is pruned at the shipped threshold, and when agreement
with the labels drops under its floor.

The ladder is visible while it fills: the status line shows the live load against the usable
window with its ratio and the tier that load has reached (`↕120k/160k · 0.75 · soft`), and `/status` adds the
window it came from with the reserve held back (`window · 200k raw, 160k usable, 40k reserved for
the answer`), the
accumulated ledger's size, the tier of the last pass that wrote it, and the judge while it is on —
the one setting here that leaves the machine is named where a reader already looks.

## Context and skills

Five things the TUI adds beyond flags and the model's own tools, all client-side, none of them
in the core `nacelle` package — the same "the library must never read configuration from disk"
rule above applies to these too, not only to settings.

**This session's own facts** (always on, `environment.go`). Prepended to whatever `System`
says, before context and skills, because it is the most general layer and the only one with no
switch: the absolute working directory, that the file and search tools are confined to it and
strip a leading `/` rather than refusing it, today's date, that the answer is rendered as
markdown, and whether anyone reviews a tool call before it runs. Two clauses are conditional the
way the banner's own lines are — the `run_command` path exception and the paragraph naming
irreversible commands appear only under `-bash`, because explaining a tool the model was not
given teaches it to skim the rest. These are facts the model cannot recover from any tool schema.
Advice about *which* tool to reach for is deliberately not here; it lives in the tool
descriptions, where it travels with the tool that needs it and costs nothing when that tool is
not mounted.

The persona above it (`defaultSystem`, `main.go`) is the one layer `-system-prompt` replaces
outright, and it stays deliberately short. Codex ships two prompts for one harness — 6.6&nbsp;KB
for the models post-trained on it, 24&nbsp;KB for general GPT-5 — so prompt size mostly measures
how much the model was *not* trained on your harness, and Claude is well inside the trained case
for this shape of tool. What it carries is only what a terminal changes about answering: brevity,
paths rather than pasted source, and not claiming something works unchecked.

**Your own steering text** (`-additional-prompt`, `session.additional_prompt`) is appended last
of all, after the skills catalog. It is the one prompt setting that composes rather than replaces:
in a transformer prompt later text carries more weight, so a directive placed at the end exerts
the most influence over the guidance it sits on — and it mirrors how `CLAUDE.md` and `AGENTS.md`
already work, appended at the end as user instructions layered over the base. Use it for a persona
that must steer every turn, which a skill cannot do: skills use progressive disclosure, so only
their name and description reach the prompt and the body is read on demand. Leaving it empty is
the ordinary case.

**Project and global context** (`-project-context`, `context.go`). Every `CLAUDE.md` and
`AGENTS.md` found walking up from `-root` to the filesystem root, plus `~/.agents/AGENTS.md` —
the [AGENTS.md standard](https://agentsstandard.com)'s own global-base path, also read by
Codex, Cursor, Copilot, Gemini and pi (at its own equivalent, `~/.pi/agent/AGENTS.md`) — all
appended to the system prompt, most general first. A user's own `~/.claude/CLAUDE.md` is
deliberately **not** read: it assumes Claude Code's own tools, hooks and slash commands, where
an `AGENTS.md` at the standard's own path is written, by the convention's premise, to make
sense to whichever agent finds it. None of this is trust-gated — see
[architecture.md](architecture.md) for why a plain instruction file and a skill are not the
same risk.

**Skills** (`-skills`, `-trust-skills`, `-skill-dir`, `skills.go`), following the
[Agent Skills specification](https://agentskills.io/specification): every `SKILL.md` under
`~/.agents/skills/` (no trust needed, the user's own machine), every one under a **trusted**
`.agents/skills/` found the same way the context walk works, and every one under a directory
named by `-skill-dir` (repeatable), `BULLE_SKILL_DIRS` (colon-separated) or `skill_dirs` in
`~/.bulle.yml`. Only a skill's `name` and `description` ever reach the system prompt; the
model reads the rest with `read_file` once it decides a skill applies. `-trust-skills` trusts
every project-local `.agents/skills/` found on that run and remembers the decision in
`~/.bulle/trust.json`, keyed by canonical directory — run it once per project, not on every
launch.

**MCP servers** (`mcp.go`). A server is written inline under `sources.mcp:` in `~/.bulle.yml` — nothing
else is needed to start one:
```yaml
sources:
  mcp:
    mycelium:
      command: mycelium
      args: [mcp]
```
The key is the server name;the definition uses the keys the `mcpServers` format spells — `command` + `args`
(stdio), `"type": "http"` + `url` (remote), `env`, `cwd`, `headers`, `disabled`. Both stdio and HTTP
servers work, on **either backend**:the tools are bridged to `nacelle.Tool`, so
`-approve-tools` gates them like every other tool and `-backend openrouter` gets them too. `${VAR}`和
`${VAR:-default}` expand,, so a token stays in the environment rather than in `~/.bulle.yml`. Writing
the definition here means there is no second file to keep in step and no pointer to remember to point tor

For the times a server is already configured for another client, `-mcp <file>` (repeatable) names
a `.mcp.json` in the `mcpServers` format and merges its servers in — by server name,, with the flag's
servers winning over the inline ones,, so `-mcp ~/.claude/.mcp.json` reads what Claude Code already
has without copying it. Replacing would mean naming one server silently switching off nine already
configured; merging is how a personal inline list plus another client's file stack the way every client
in this ecosystem layers its own scopes.

A server that will not start **ends the run**, which is the opposite of how skills and project
context fail. Those are discovered, so finding nothing is indistinguishable from there being
nothing to find; this was asked for by name, and a tool that is quietly missing reads as a model
refusing to work rather than as a server that is down.

Nothing is discovered. A `.mcp.json` sitting in the working directory is **not** read, because it
names executables to run — strictly worse than the project-local `.agents/skills/` this client
already gates behind `~/.bulle/trust.json`, since a skill is text the model may decline to act
on and this is a subprocess started before the model is asked anything. Every server bulle
opens was named on the command line or in your own `~/.bulle.yml`.

The spec defines the `SKILL.md` file, not where it has to live on disk — every tool picks its
own directory (Claude Code reads `~/.claude/skills/`, pi reads `~/.agents/skills/` and its own
`~/.pi/agent/skills/`), so `~/.agents/skills/` is bulle's own choice, not a location any other
tool already reads. `-skill-dir` is how bulle sees a directory that belongs to one of those,
without moving or copying anything into its own — the same problem pi itself solves with a
`skills` array in `settings.json` pointed at `~/.claude/skills` or `~/.codex/skills`. No trust
gate applies to it, same reasoning as `~/.agents/skills/` above: naming a directory here is
something only the person running bulle, on their own machine, can do in the first place.

**The banner** (`main.go`'s `banner`) is how much of this is actually visible before typing
anything: line one names the backend and model, line two the resolved `-root`, how many skills
loaded (from every source above, combined), how many `CLAUDE.md`/`AGENTS.md` files did, and
whether bash is on. Nothing on it is decorative — each answers a real "is that actually on"
question this client otherwise had no way to check short of a debug build.

`bash off` earns its place because the symptom arrives from the model rather than from this
client: asked to build and run something, it answers that it has no terminal and cannot run a
command. That is true and deliberate — `run_command` is unconfined, so it stays opt-in — but
nothing connected that answer back to a `tools.run_command: false` written once in `~/.bulle.yml` and
forgotten. Turn it on with `-bash`, `BULLE_BASH=1`, or `tools.run_command: true`.

## Reading a page

`tools.web_fetch` (`BULLE_FETCH`, `-fetch`) lets the model read one web page by URL. **On by default**,
unlike bash, and it is what turns a page's URL into text the model can act on.

It is on by default because it cannot change anything and cannot reach anything but the public
internet: loopback, private ranges, the cloud metadata endpoint and the special-use ranges are
refused in the dialer, on every redirect hop. Pages come back as text — headings, lists, code
blocks, and links resolved to absolute URLs so the model can follow one by calling the tool
again. `text/markdown` is asked for first, which Cloudflare and Vercel answer by converting at
the edge for roughly 80% fewer tokens.

Turn it off with `web_fetch: false` for the one risk the guard cannot cover: a fetched page is
written by a stranger and read by the model as instructions, so it can ask for another URL with
something from the conversation in its query string. With `tools.run_command: true` that channel already
exists through `run_command`; with bash off, this is the only one. The banner says `fetch off`
when it is off, since a model that cannot read a page it just found needs the reason on screen.

Sites do refuse automated clients. This one identifies itself honestly rather than impersonating
a browser — every current source agrees that negotiated access is what gets an agent through and
that a spoofed user agent is what gets an address banned — so a 403 from an anti-bot layer is
reported as one, with the suggestion to try another source, rather than as the page not existing.

## Tool approval

`-approve-tools` (`BULLE_APPROVE_TOOLS`, `approve_tools`) asks before every tool call runs.
**Off by default**: a nil `Config.Approve` means the SDK itself runs every call unasked, and
the TUI's own default matches — nobody gets a behaviour change without opting in.

When on, a call blocks in the status line on `y` (allow this one call), `a` (allow this tool
for the rest of the session — remembered in memory only, never written to disk, unlike
`-trust-skills`) or `n` (deny; asked again next time). Asks are serialized to one question at
a time, because a backend can run several tool calls concurrently within a turn and this
client has exactly one place to show a question. A pending question is answered or abandoned
by the same ctrl+c/ctrl+\ that already gets a wedged run's way out — both wait on the run's
own cancellable context, so cancelling one unblocks the other with no separate mechanism.

A denial is reported on `ToolEvent.Refused`, not as a tool failure — the SDK converts it into
a normal tool-result error block for the model to see, the same as any other failed call.

## Deny elevation

`security.deny_elevation` (`BULLE_DENY_ELEVATION`) refuses `run_command` calls that try to
elevate privileges: `sudo`, `su`, `doas`, `pkexec`, checked after every chain separator and
inside subshells, not only at the start of the line. **On by default.** The refusal is an
ordinary tool error the model reads, so it adapts instead of retrying.

It is a policy guard against accidents and injected instructions, not a sandbox. A string
check cannot stop a setuid binary that is not sudo, `docker run -u root`, or anything already
running as root; the OS decides who may elevate, and an account outside the `sudo` group is
the actual wall. Set it to `false` to allow elevation. Phase 2 adds a pty password flow, where
sudo prompts on a terminal the harness allocates but never reads, so the agent never sees the
password. [docs/sudo.md](sudo.md) carries the full analysis.

## Cron jobs

Cron is bulle run unattended: the same agent, driven headless by an external
scheduler — a systemd timer or a crontab — while nobody watches. bulle owns
the job definitions and the run itself; the clock belongs to systemd, which is
why there is no daemon here to reload or supervise. The `bulle cron`
subcommand ties the two together:

```sh
bulle cron list            # every job, trusted or not
bulle cron run news        # run one job now, through the headless path
bulle cron install news    # install the job directly into crontab
bulle cron uninstall news  # remove the job from crontab
bulle cron trust news      # approve one job file's current contents
```

`bulle cron list --json` prints the same table as one JSON document; set
`ui.cron_list_json: true` to make that the default output.

### The jobs folder

Each job is one YAML file in `~/.bulle/jobs/` — `news.yml`, `healthchecks.yml`,
one file per job, the job's own fields at the top level of the file rather than
wrapped in a list. The job's name defaults to the filename stem, so `news.yml`
defines the job `news` and `bulle cron run news` runs it; a `name:` key
overrides it for the rare file whose stem is not the name you want (a name
must be unit-safe: letters, digits and `. _ -` only).

The folder is scanned on every `cron` invocation, list included, so there is no
daemon and no reload step: a file dropped in appears in `cron list` on the next
invocation, a file edited changes the job, and a file deleted takes the job
with it. Deleting the file is the off switch.

Job files decode strictly, the way `~/.bulle.yml` itself does: a typo in a
job file is a hard error naming the file, not a silent half-job. Loud is the
right setting for something that fires at 2am.

### Trust

A job file is instructions an agent will follow unattended with your provider
keys in reach, so it is gated the way project skills and hooks are: a human
must approve the exact bytes before they run.

- A new file shows up in `bulle cron list` immediately, marked untrusted.
- `bulle cron run` and `bulle cron install` refuse an untrusted job until
  someone runs `bulle cron trust <name>`, which prints the file's path, its
  name and its prompt, and asks before recording anything.
- Trusting records a SHA-256 hash of the file's contents against its path in
  `~/.bulle/trust.json`, the same store that remembers approved skill
  directories. Editing the file changes the hash, and a changed file is
  refused again — trust re-arms on every edit, so no one-line change to a
  trusted job slips through unreviewed.
- Trust is per machine. A `~/.bulle/jobs/` folder synced across machines
  carries the files but not the approvals, so each machine needs its own
  `bulle cron trust` — the person trusting is a person at that machine's
  keyboard, and that is the point.

Trusting makes a job **runnable**, not **scheduled**: it clears `cron run` and
unblocks `cron install`, and `cron install` still only prints the systemd
service and timer units for you to save and enable yourself. Nothing bulle
does creates the timer.

### A job file

```yaml
# ~/.bulle/jobs/news.yml — one file, one job; the job's name is the stem.
when: daily
prompt: |-
  Summarise what changed on the pages in my feed list since yesterday.
  Keep it under 200 words and name the two most important changes.
workdir: ~/Code/Facile/notes
delivery: file:~/notes/cron
timeout: 600
enabled: true

provider:
  backend: openai
  model: gpt-5.4

security:
  deny_elevation: true

tools:
  run_command: true
  web_fetch: true

reasoning:
  effort: low

limits:
  max_iterations: 5
```

| Field | Default | What it does |
|---|---|---|
| `when` | empty (`install` writes `daily`) | The `OnCalendar=` line of the generated timer — any systemd calendar expression. bulle never parses it; systemd does |
| `prompt` | — required | The question the run starts from |
| `workdir` | — install requires one | The run's root and the unit's `WorkingDirectory=`. Without one the run would execute wherever the scheduler happens to start the unit, which is never the project |
| `delivery` | empty — no transcript written | `file:<dir>` appends the transcript to `<dir>/<name>-YYYY-MM-DD.md` and one status line per run to `<dir>/<name>.log` |
| `timeout` | `300` | The service's `TimeoutStartSec=` — systemd kills a run outliving it |
| `enabled` | `false` | `cron install` refuses a disabled job, so test-run-first is explicit: `cron run` it once, read the output, then set `enabled: true` |
| `name` | the filename stem | Overrides the job's name |
| `commands` | `false` | The legacy spelling of `tools.run_command`, kept from the inline-list era for files that already use it |

### Per-job overrides

Every group the interactive config has can be overridden per job: a job can run
on a different backend than the interactive one, tighten its own isolation, or
cap its own spend. Members are per-field — mention a group with one key set and
every key you leave out keeps the config's value; a job saying nothing runs
exactly on the config's settings.

| Group | Keys | Notes |
|---|---|---|
| `provider:` | `backend`, `model`, `base_url`, `api_key` | A different backend, endpoint or key for this job — the cheap model for the dawn summary, the local gateway for the nightly audit. `provider.model` wins over the flat `model:` when both are set |
| `security:` | `approve_tools`, `path_isolation`, `deny_elevation`, `env_isolation` | `approve_tools` is forced off for every cron run whatever the job or the config says — a run nobody watches cannot answer a prompt, and a prompt nobody answers is a run that never starts. The other three apply as written |
| `tools:` | `run_command`, `parallel_agents`, `web_fetch`, `tasks`, `diagnostics` | `run_command` defaults to **false** for cron, the reverse of the interactive default: opt in with `tools.run_command: true` or the legacy `commands: true`, and `tools:` wins when both are spelled |
| `reasoning:` | `effort`, `thinking`, `budget` | Per-job reasoning spend — `low` for the summariser, `high` for the auditor |
| `limits:` | `max_iterations`, `compact_at`, `compaction` | Iteration and compaction caps that only make sense per job; `compaction` merges member by member like every other group |
| `hooks:` | hook specs, same shape as the config's own | **Append** to the config's hooks — a job adds chains on top of the ones every run gets, it never replaces them |
| `gates:` | gate specs, same shape as the config's own | **Append** to the config's gates, same rule |

### Migration

The inline `cron:` list in `~/.bulle.yml` is gone. A config still carrying it
is refused at load with an error naming `~/.bulle/jobs/` — the same refusal an
unknown key gets, because a silently ignored job list is a scheduler that
quietly stopped existing. Each list entry becomes its own file in the folder:
the entry's `name` names the file (`news` becomes `news.yml`), the rest of the
entry moves across unchanged, and `commands: true` keeps working as the legacy
spelling of `tools.run_command: true`.

## Chat

`bulle chat` is the inbound surface: one long-lived process holding a Matrix connection, so
a message from an allowlisted account runs a bulle session and the answer comes back in the
same room. Every other part of bulle pushes text out; this is the one part that reads text
in. The design, the evidence and the deliberate omissions are in
[plan-matrix-chat.md](plan-matrix-chat.md); this page is the settings and the first run.

It is cron's opposite in one respect. A [cron job](#cron-jobs) is fire-and-forget, armed by
systemd and run once; chat is a supervised service that has to stay up to hear anything.

```sh
bulle chat              # hold the connection in the foreground
bulle chat channels     # what is configured, and its state
bulle chat channels --json
bulle chat install      # write and start the systemd user unit
```

`bulle chat` holds the connection until it is stopped. `bulle chat install` writes a systemd
**user** unit and enables it, so the process survives logout; on a machine with linger
already on it also starts at boot. The unit restarts on failure rather than always: a
revoked token would restart in a loop forever under `Restart=always`.

### The chat block

Everything lives under `chat:` in `~/.bulle.yml`. It is off until `enabled` is `true`, and an
untouched file with it off opens no connection.

```yaml
chat:
  matrix:
    enabled: true
    homeserver: https://matrix.example.org
    user_id: "@bulle-bot:example.org"
    device_id: JLAFKJWSCS
    # access_token: ""            # empty plus the command below keeps the token here unset
    access_token_command: tiroir get BULLE_MATRIX_TOKEN
    # password: ""                # the alternative to a token, see "Choosing how the bot authenticates"
    # password_command: tiroir get BULLE_MATRIX_PASSWORD
    # pickle_key: ""              # empty plus the command below keeps the key here unset
    pickle_key_command: tiroir get BULLE_MATRIX_PICKLE_KEY
    allow:
      - "@you:example.org"
    rooms:
      - "!roomid:example.org"
    max_age: ""
    workdir: ""
```

The defaults for the two settings without a natural fixed value are both **empty**:
`max_age` empty keeps every message, and `workdir` empty uses the daemon's own working
directory, which is what the unit's `WorkingDirectory=` sets. `max_age` takes a duration
(`1h`, `30m`) and drops anything older, so a restart does not answer a stale backlog.

### Choosing how the bot authenticates

There are two ways in, and they are alternatives rather than layers. Pick one and stay on it.

**Password login** is the easier first run, because bulle logs in itself and the crypto helper
creates the device, so there is no device to name beforehand.

```yaml
chat:
  matrix:
    user_id: "@bulle-bot:example.org"
    password_command: tiroir get BULLE_MATRIX_PASSWORD
```

`user_id` takes the full MXID or just the localpart (`bulle-bot`); the homeserver accepts both
for `m.login.password`, and the login response supplies the full ID either way. On the run that
creates the device bulle names it `bulle`, which is what you will see in Element's device list.
Afterwards the homeserver ignores that name, so a restart does not rename a device you have
already recognised.

**An access token** authenticates without a login, and is the arrangement to move to once the
password route works: nothing to store but the token, and no login on every start.

```yaml
chat:
  matrix:
    user_id: "@bulle-bot:example.org"
    device_id: JLAFKJWSCS
    access_token_command: tiroir get BULLE_MATRIX_TOKEN
```

`device_id` is required on this route when the crypto database is empty, because the token
belongs to a device and bulle has to know which one. That is the run that creates the database,
so on a fresh install you need both fields from your own login:

```sh
curl -s https://matrix.example.org/_matrix/client/v3/login
# {"flows":[{"type":"m.login.password"}]}

curl -s -X POST https://matrix.example.org/_matrix/client/v3/login \
  -H 'Content-Type: application/json' \
  -d '{"type":"m.login.password","identifier":{"type":"m.id.user","user":"bulle-bot"},"password":"…"}'
```

```json
{"user_id":"@bulle-bot:example.org","access_token":"syt_…","device_id":"JLAFKJWSCS"}
```

Those three response fields are the three config keys: `user_id` goes to
`chat.matrix.user_id`, `access_token` to `chat.matrix.access_token`, and `device_id` to
`chat.matrix.device_id`. An existing token works too, and `/account/whoami` tells you which
`user_id` and `device_id` it belongs to:

```sh
curl -s https://matrix.example.org/_matrix/client/v3/account/whoami \
  -H 'Authorization: Bearer syt_…'
# {"user_id":"@bulle-bot:example.org","device_id":"JLAFKJWSCS"}
```

**Do not set both.** A token always wins, so a leftover token silently keeps being used after
you switch to a password. There is a second reason to keep to one route: the homeserver treats
a login as authoritative for its device and invalidates that device's previous access token,
so once the password route runs, any token you exported for the same `device_id` stops working.
That is expected, not a bug in bulle.

An existing token works too. The one Element uses is a valid access token; ask the
homeserver whose it is:

```sh
curl -s https://matrix.example.org/_matrix/client/v3/account/whoami \
  -H 'Authorization: Bearer syt_…'
# {"user_id":"@bulle-bot:example.org","device_id":"JLAFKJWSCS"}
```

`whoami` is how a token from another client is turned into the `user_id` and `device_id`
bulle needs. A token and a password are alternatives, not layers: set one route or the other.

### Keeping the secret out of the file

`access_token_command`, `password_command` and `pickle_key_command` are the same arrangement
`provider.api_key_command` has: the command prints the value on stdout, so the config file
carries no secret and can be committed.

```yaml
chat:
  matrix:
    enabled: true
    homeserver: https://matrix.example.org
    user_id: "@bulle-bot:example.org"
    device_id: JLAFKJWSCS
    access_token_command: tiroir get BULLE_MATRIX_TOKEN
    pickle_key_command: tiroir get BULLE_MATRIX_PICKLE_KEY
```

Each command runs through `sh -c` and its stdout is trimmed; a command that fails is refused
rather than left as an empty value.

### The crypto database

Matrix end-to-end encryption keeps a device's keys, and bulle keeps them in a sqlite database
under `~/.bulle/chat/`. The **pickle key** encrypts the stored device keys at rest, so losing
it is the same as losing the database: bulle comes up as a new device, which Element shows as
a new unverified device. Back the pickle key up somewhere other than beside the database. The
plan names a backup story as an open risk; there is none beyond that yet.

### End-to-end encryption

bulle uses `mautrix-go` and encrypts with Olm and Megolm. The release build sets
`CGO_ENABLED=0` and the pure-Go implementation of Olm is selected with the `-tags goolm`
build flag, because the default imports `libolm`, a C library, and the cross-compiled release
cannot link it. For the same reason the crypto store is `modernc.org/sqlite`, a pure-Go
driver.

goolm builds for every release target and passes its own test suite. It is **not audited
upstream**, so the encryption is only as trustworthy as an unaudited implementation allows.
Nothing in this repository has been tested against a live homeserver yet: the encrypted path
has never decrypted or sent a real event here.

An unverified bot device decrypts only while the sender's client chooses to share room keys
with unverified devices, which is Element's default today. When a sender's client stops doing
that, the room key is withheld and the bot reads nothing. Set `self_sign: true` to fix that
without a human tapping emoji in Element: on the first run the bot generates its own
cross-signing identity, publishes it, and signs its own device.

```yaml
chat:
  matrix:
    self_sign: true
```

Self-signing writes account data on the homeserver, so it stays off until you ask. It runs
once, and it refuses to run again on an account that already has cross-signing keys, because a
second identity would un-verify every device signed by the first. The run that creates the
identity prints a **recovery key** once and writes it to `~/.bulle/chat/recovery.key` with mode
0600. Keep it: the signing keys otherwise live only in the account's server-side secure secret
storage, and the recovery key is the only way to get them onto a fresh database.

A homeserver that demands a password to publish keys (`Synapse` does, for anything past a
first master key) needs `password` or `password_command` set. With a token and no password,
self-signing fails and says so.

If the account already has cross-signing keys (for example, configured via Element), provide
the recovery key via `chat.matrix.recovery_key`, `chat.matrix.recovery_key_command`, or run:

```sh
bulle chat verify --recovery-key "EsTE s92N ..."
```

bulle verifies the device against the existing cross-signing identity and signs it.

Replies thread onto the message that triggered them with `m.in_reply_to`, so an answer attaches
to its question in a busy room. An answer too long for one event is split across several
messages, and only the first carries the reply relation. A room that gets upgraded does not
re-key itself: the room ID changes, the conversation is over, and bulle logs the replacement
room ID so you can decide whether to add it to `chat.matrix.rooms`.

### The allowlist

Two lists decide who may make this machine run a tool. `chat.matrix.allow` holds MXIDs that
may start a session; `chat.matrix.rooms` holds room IDs that are read at all.

**Both empty refuses every message.** That is the default and the right one: bulle runs in
your repos with shell access, so the allowlist is the security boundary of the whole feature,
not a filter on top of it. The check runs **before** the agent is built, never inside the
run: an agent asked to refuse its own input has already been given the input.

Every inbound message is untrusted text, whoever sent it. The chat path never touches
`approve_tools`, so the run inherits whatever the operator set, and the two settings behave
very differently here:

- **`approve_tools: false` (the default)** means no tool call asks for approval, so a message
  from an allowlisted account runs tools unattended, including `run_command`. Nobody is
  watching the terminal, so this is the posture to understand before pointing a room at bulle:
  the allowlist is the only gate between a message and a shell.
- **`approve_tools: true`** fails closed instead. `approval.Build(true)` with no UI wired
  returns false from `Ask`, so a run that would need approval is denied rather than prompting
  into the void. A chat-started run then does nothing until you answer on the workstation.

Either way, do not treat a room as a sandbox. Set `root` to the directory a chat run may
reach, and leave `path_isolation` on.

### Out of scope

A second adapter, routing chat through Antenne, approval buttons in chat, message history as
a store, and more than one machine answering as one bot are all deliberately not built. The
reasons are in the plan's [Skip section](plan-matrix-chat.md#skip-yagni).

## Slash commands

Typing `/` at the start of a line names one of the client's own commands instead of a
question:

| Command | Does |
|---|---|
| `/clear` | Reset the transcript, the conversation sent to the model, and the running cost total. Same client, new session. Never starts a run. |
| `/cost` | Display cumulative token usage, cost, and tools executed so far. Never starts a run. |
| `/help` | List the commands above and the keybindings (esc, ctrl+c/ctrl+\, ctrl+t, ctrl+g). Never starts a run. |
| `/model [name]` | Switch active model or profile mid-session, or open model picker. Never starts a run. |
| `/quit` | Quit. Never starts a run. |
| `/resume` | Resume the most recent recorded session for the current project. Never starts a run. |
| `/sessions` | List recorded sessions for the current project with timestamps and models. Never starts a run. |
| `/status` | Show runtime status, context usage, cost, and active configuration. Never starts a run. |
| `/skill:name [what to do]` | Run a loaded skill directly. Starts a run, unlike the seven above. |

An unrecognised command or skill (a typo like `/clera`, or a `/skill:name` that names nothing
loaded) is reported back rather than sent to the model as a literal question — the same
trade-off every peer client with slash commands makes, on the same reasoning: a real question is
far less likely to start with a slash than a mistyped command is.

`/skill:name` sends that skill's own `SKILL.md` content as the question, instead of waiting for
the model to decide on its own that the skill applies and read it with `read_file` — the same
shortcut pi's own `/skill:name` offers, for the same reason: "models don't always" make that
call themselves. Anything typed after the name is appended as `User: <that text>`, the same
convention pi uses, so `/skill:pdf-tools extract the tables` tells the skill what to do with
itself rather than only that it applies. Every loaded skill counts, from `~/.agents/skills/`,
a trusted project `.agents/skills/`, or a `-skill-dir` — one namespace, `skill:`, so a skill
named `clear` or `help` can never collide with the three commands above.

Typing `/` alone opens a dropdown listing every command and skill, each skill with its own
description; more typed narrows it, ranked best match first — a prefix beats a plain substring
beats a fuzzy, order-preserving scatter of the same letters, so `/skill:rev` finds `review`,
`facile-review` **and** `hunk-review`, not only a name that starts with what was typed. While
it's open, `up`/`down` move the selection instead of scrolling the transcript, `tab`/`enter` fill
the prompt with the highlighted entry (plus a trailing space, and without submitting — most
useful with `/skill:name`'s own trailing argument) and `esc` closes it, leaving what was typed
alone. A second, ordinary `enter` is what actually sends it.

### The prompt

The prompt wraps and grows as you type, up to ten rows, then scrolls inside itself. Every row it
gains is a row the transcript gives up, and gets back the moment the question is sent. `alt+enter`
(or `ctrl+j`) starts a new line without sending; `enter` always sends.

It used to be a single-line input, which scrolled sideways instead of wrapping — a question wider
than the terminal slid out of view a character at a time and read as though it were typing over
itself.

`up`/`down` belong to the prompt at every height — there is no transcript here to scroll, so
nothing competes for them and a wrapped question is editable whatever size it has grown to.

### Scrolling

The client renders inline, on the terminal's own screen, so scrolling is the
terminal's job — wheel, scrollbar, `tmux` copy-mode, your terminal's search, all of it, on the
whole session rather than on a window this client owns. There are no scroll keys here because
there is nothing here to scroll.

That is a deliberate reversal. It ran on the alternate screen once, which hands a program a
blank page with no scrollback of its own, so the client had to own scrolling: capture the wheel,
which takes click-drag selection with it, leave copy-mode reaching nothing, and un-draw the whole
session on quit. Those arrived as four separate complaints and were one decision. Giving the page
back answers all of them, and costs one thing — a printed line belongs to the terminal, so a
resize reflows it the terminal's way rather than this client's, and a theme change applies from
then on rather than retroactively.

### While a run is going

The status line spins for as long as the run lasts, and names what it is waiting on — `waiting for
a response` before the model answers, `running read_file` between a tool's call and its result,
`running 3 tools` when the backend runs a turn's tools at once. It is on the one row that is
always drawn, so it stays visible while scrolled back through the transcript.

This replaced a spinner that only covered the gap before the first event. Every gap after it — a
tool running, the model called again with its result — left the screen still, and a client that has
stopped moving reads exactly like one that has stopped working.

**Enter still works while a run is going**: the line is queued, shown above the prompt, and sent
once the run finishes. A queued `/help` is still a command.

**`esc` stops the run**, and that is all it ever does. `ctrl+c` stops one too, but it is also the
key that quits an idle client, so the press that abandons an answer is a press that has to be
thought about first; `esc` is the one that never is. Either way the queue goes with the run rather
than being started by it, and the client says how many it dropped — the alternative is a message
that silently never gets asked. Idle, `esc` does nothing at all, and with the `/` dropdown open it
closes that first: one visible thing per press.

The question is on the screen before either of them is worth pressing. It used to appear only when
the answer did: the client printed what it had said only after running what that message started,
and what a question starts is a wait on the model. The prompt emptied and nothing else happened,
which reads as a client that swallowed it. The `⏺` line naming a tool had the same fault and was
worse for it — it waited on the tool it was announcing, so a six-second command was announced six
seconds late. The CHANGELOG carries the measurements.

### What a tool call reads as

A tool that worked is **one line**, carrying its own duration:

```
⏺ read_file(view.go) · 12ms
⏺ run_command(go test ./...) · 2.34s
```

The name, the single argument that says which thing it acted on, and how long it took. The
argument is picked by key (`path`, `file_path`, `file`, `command`, `pattern`, `query`, `url`,
`name`), falling back to the sole key when a call has exactly one and to `name()` when nothing
there is worth printing. That beats a table of tool-name to argument-name, because this client
does not know which tools an agent was built with and half of them arrive over MCP under a
server's own names.

The line is held until the result arrives, which is the only way the duration can be on it: a
printed line belongs to the terminal's scrollback and can never be rewritten. Nothing is hidden
while it waits, because the status line names the running tool for exactly as long as the line is
held. A run that ends with calls still in flight (the iteration limit, or `esc`) still prints
them, without a duration.

**A failure stays two lines and stays loud**, because it is the thing a reader must not scroll
past:

```
⏺ edit_file(transcript.go)
  ⤷ edit_file failed after 3ms: no such file
```

What this replaced printed the raw JSON input and then a second `done in 12ms` line under it,
so every transcript was twice as tall in order to report that nothing had gone wrong.

**A file edit also draws its diff.** When `edit_file` or `write_file` changes a file, the
one-line report is followed by what changed, git-style — removals in the terminal's red,
additions in its green, with three unchanged lines around each block so the eye can find where
in the file it is looking:

```
⏺ edit_file(toolline.go) · 4ms
   1   func (m *model) finished(tool *nacelle.ToolEvent) {
   2 -   line := m.run.running[tool.ID]
   2 +   line, held := m.run.running[tool.ID]
   3       delete(m.run.running, tool.ID)
```

Line numbers run down the left gutter — green on additions, red on removals, muted on context.
Added and removed lines each sit on a faint wash (the pure green or red blended to ~20% onto the
pane's backdrop) rather than a solid block, so a long change stays readable without shouting.
The diff is cut to the window width so nothing wraps in scrollback, and capped at four hundred
lines with a `… more` marker saying it was cut rather than pretending it wasn't. A wholesale
rewrite too large to align line by line reads as one block of removals against additions.

`edit_file` carries both sides of the change in its own input, so nothing is read from disk to
draw it; `write_file` carries only the new contents, so what the file held when the call was seen
becomes the before side — empty for a file being created, which shows as all additions. Only
these two tools get a diff: search, commands and MCP tools render exactly as they did before.
Turn it off with `diffs: false` (`BULLE_DIFFS`, `-diffs`) to restore the bare one-line report.

**Input with a repeated key is refused rather than summarised.** `encoding/json` keeps the last
value of a duplicate key silently, so `{"command":"ls","command":"rm -rf /"}` can be shown as
`run_command(ls)` over a call that ran something else. The line says `input has a duplicate key`
instead, and with `-approve-tools` on the gate denies the call outright without ever drawing a
prompt: an approval prompt that offers `y` for a call nobody can render is the same lie with a
warning label.

### Reasoning

With `thinking: true`, a turn's reasoning collapses to one quiet line:

```
· thought for 4.2s · ctrl+t to expand
```

The hint appears once per session, not under every answer. **`ctrl+t` expands**: it prints the
retained reasoning below, and sticks, so later turns print theirs in full until it is pressed
again. Expanding cannot replace the collapsed line already on screen for the same reason the
tool line carries its own duration, so a second copy underneath is what expanding means here.
Only the most recent turn's reasoning is retained. `ctrl+t` takes the press whatever is on
screen, which costs the prompt its `transpose-character-backward` binding.

### Quitting

`ctrl+c` when nothing is running, `ctrl+\` at any time, and `/quit` all end the session, and the
session simply stays where it is. Nothing is printed on the way out and nothing needs to be: what
was said went into the terminal's scrollback as it happened, so quitting leaves it exactly as
running left it.

That used to need a deliberate dump at exit, because the alternate screen hands the old page back
untouched and quitting un-drew the whole conversation. Inline rendering removed the problem rather
than the workaround.

## Sandboxes & remote hosts

bulle always runs **on the host**, with the session's tools executing in the
target over SSH. The model reaches the target's workspace, but no provider key,
session recording or host file ever crosses the boundary — the tools are the
only thing that does. Two groups configure the two kinds of target:

```yaml
sandbox:                          # local boite microVMs, used by 'bulle sandbox'
  default: ""                     # VM entered when 'bulle sandbox' has no argument
  user: boite                     # default SSH user
  port: 2226                      # default port
  ssh_key_path: ~/.ssh/id_ed25519 # default identity file
  root: ""                        # empty uses the VM's own workspace
  auto_snapshot: false            # snapshot the overlay disk on exit
  targets:
    dev-vm:
      vm_name: dev-box            # boite instance behind the key (default: the key)
      port: 2226
      user: boite
      ssh_key_path: ~/.ssh/id_ed25519
      root: ""
      auto_snapshot: false

remote:                           # SSH hosts, used by 'bulle remote'
  default: ""                     # host entered when 'bulle remote' has no argument
  user: ""                        # default SSH user (ssh's own config wins when empty)
  port: 0                         # 0 lets ssh choose: ssh_config's Port, then ssh's default (22)
  ssh_key_path: ""                # empty lets ssh choose: ssh_config IdentityFile, then the agent
  root: ""                        # empty starts in the SSH login directory (home)
  targets:
    staging:
      host: staging.example.com
      port: 22
      user: deploy
      ssh_key_path: ""
      root: /srv/app
```

Host-key verification is deliberately not relaxed for `remote`: a host on the
network checks against your own `known_hosts` and `~/.ssh/config`, exactly as a
hand-typed `ssh` would, and `BatchMode=yes` refuses an unknown host rather than
trusting it. Because `BatchMode` also removes the yes/no prompt a hand-typed
ssh shows, the first connection to an unknown host fails; that failure names the
host and the `ssh-keyscan` command that trusts it, so the step stays explicit
rather than automatic. Boite microVMs are the exception — their key is generated
per instance, never in `known_hosts`, and the connection is loopback — so only
the `sandbox` path turns host-key checking off. A `remote` target that happens
to be a local boite VM is better run through `bulle sandbox`, which trusts the
VM's key without touching `known_hosts`.

Commands:
- `bulle sandbox list`: lists configured `sandbox.targets` and local boite VMs.
- `bulle sandbox <vm> [prompt]`: interactive or headless session inside a boite VM.
- `bulle sandbox <vm> --snapshot`: snapshot the VM's overlay disk when the session ends.
- `bulle remote list`: lists the hosts configured under `remote.targets`.
- `bulle remote <host> [prompt]`: interactive or headless session on an SSH host.

`remote` accepts a `remote.targets` key, a direct `user@host:port` address, or
an `~/.ssh/config` host alias — ssh itself resolves the last two. Everything
else (port, user, identity, workspace) comes from the `remote` group unless the
target or a flag overrides it. Overrides common to both commands:
`--workdir/-w`, `--user/-u`, `--print/-p`; `remote` adds `--port` and `--key`.

`sandbox.targets` is boite-only. An SSH host belongs under `remote.targets`;
a key there that names a `backend:` or `host:` is refused at load, because a
strict decoder only accepts the fields each group actually has.

Preflight checks who the target answered as, and refuses a mismatch. It does not
refuse `root`: `bulle remote root@host` works, because connecting as root to a
machine you own is your call. A boite VM still resolves to `boite` unless you
override `--user`, so an instance that unexpectedly lands you as root is caught
by the user check rather than by a name.

