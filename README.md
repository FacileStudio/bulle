# bulle

Terminal coding agent — the human harness for the
[nacelle](https://github.com/FacileStudio/nacelle) agent SDK.

The binary is named `bulle`. The SDK is a Go library for building agents;
this program is its first consumer and lives to exercise every part of it from
a terminal, where someone is watching: text, reasoning, tools starting and
finishing, why a turn ended, what it cost. It is deliberately small — sessions,
profiles and panes are what a product grows, not what a contract test needs.

**Note**: This repo changed its name from `nacelle-tui` to `bulle` on 2026-09-14. The core SDK (`nacelle`) remains the same library.

## What it does

- Streams one model turn at a time in a full-screen Bubble Tea v2 interface
- Runs against any backend the SDK ships: `anthropic`, `google`, `openai`, or `openrouter`
- Lets the model read and edit files under a root you choose, run commands when
  `-bash` is on, fetch web pages, and call MCP server tools from files
  every other client already has (`-mcp ~/.claude/.mcp.json`)
- Lets the model lay a large job out as steps and keep them current while it
  works, drawn live above the prompt and scrolled to the step in flight
- Fans independent side tasks out to concurrent nested runs with `-subagents`,
  so a wide search or a log dump costs the conversation one answer instead of
  its whole output
- Discovers project context (CLAUDE.md, AGENTS.md) and skills into the system
  prompt, each behind its own flag
- Gates tool calls behind an approval prompt with `-approve-tools`, and trusts
  project hook files only after an explicit, remembered decision

## Stack

| Layer | Tech |
|---|---|
| TUI | Go 1.26.4, `charm.land/bubbletea/v2`, lipgloss v2, glamour v2 |
| Agent | [FacileStudio/nacelle](https://github.com/FacileStudio/nacelle), pinned by tag |
| State | `~/.bulle.yml`, `~/.bulle/hooks.json` for hook trust |
| Release | GoReleaser, GitHub Actions on tag push, Homebrew tap `FacileStudio/tap` |

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/FacileStudio/bulle/main/install.sh | bash
```

Installs to `~/.local/bin` via [facile](https://github.com/FacileStudio/facile), the suite
installer. Pass `--bin-dir <dir>` to change that, `--source` to build from source.

Already have `facile`:

```sh
facile install bulle
```

Or Homebrew:

```sh
brew install FacileStudio/tap/bulle
```

## Usage

Run it in the directory you want it to work in:

```sh
bulle
```

It reads API keys from the environment: `ANTHROPIC_API_KEY` for the default
backend, `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) for `-backend google`,
`OPENAI_API_KEY` for `-backend openai`, and `OPENROUTER_API_KEY` for `-backend openrouter`.

Settings layer bottom-up: defaults, then `~/.bulle.yml`, then `BULLE_*`
environment variables, then flags. The useful ones:

| Flag | Env | What |
|---|---|---|
| `-backend` | `bulle_BACKEND` | `anthropic`, `google`, `openai`, or `openrouter` |
| `-model` | `bulle_MODEL` | model id; the backend's own default otherwise |
| `-root` | `bulle_ROOT` | directory the file tools may reach |
| `-bash` | `bulle_BASH` | let the model run commands (off by default) |
| `-continue` | — | auto-resume the newest session for the current project |
| `-resume` | — | resume a specific session by id or file path |
| `-no-config` | — | start with default settings, ignoring ~/.bulle.yml |
| `-tasks` | `bulle_TASKS` | task planning tool (on by default) |
| `-approve-tools` | `bulle_APPROVE_TOOLS` | ask before every tool call runs |
| `-subagents` | `bulle_SUBAGENTS` | give the model the parallel delegate tool (on by default) |
| `-max-iterations` | `bulle_MAX_ITERATIONS` | how many times the model may be asked |
| `-mcp` | — | MCP servers file (repeatable) |
| `-skill-dir` | `bulle_SKILL_DIRS` | extra skills directory (repeatable) |
| `-ide` | `bulle_IDE` | publish the session to an editor over a unix socket |

`bulle -version` prints exactly `bulle <semver>`. See `-h` for the full set:
reasoning effort and budget, web fetch, project-context and skill
discovery, hooks trust.

Full settings reference: [docs/configuration.md](docs/configuration.md).

## Configuration

Settings live in `~/.bulle.yml`, **written on first boot with every default
explicitly set** — delete it to regenerate. An existing file is never touched.
The example file with all defaults, and the full reference with the
precedence order and the traps in each setting:
[docs/configuration.md](docs/configuration.md).

`example.bulle.yml` in this repo is the same file the first boot writes — the whole
surface, abridged here. The abridgement is illustrative: the scaffold-parity test covers
`example.bulle.yml`, and nothing reads the block below, so a wrong key here fails no gate —
it fails at load, under `KnownFields`, when you paste it.

```yaml
provider:
  backend: anthropic
  model: ""
  base_url: ""
  api_key: ""

session:
  root: .
  system_prompt: ""
  additional_prompt: ""
  continue: false

limits:
  max_iterations: 5
  # compact_at is an absolute token ceiling when set, 0 disables compaction,
  # and unset derives the ceiling from the context window and the ratios here.
  compaction:
    # Two of these are yours to decide: judge.enabled below, and compact_at
    # above. The rest are defaults that are right for most sessions.
    #
    # Ratios are fractions of the window a turn can fill: the backend's window
    # less reserve_tokens, the runway held back for the model's answer.
    # soft_ratio is the free rung; smart_ratio is the paid one. Raise soft_ratio
    # before touching smart_ratio.
    soft_ratio: 0.65
    smart_ratio: 0.80
    # window_tokens overrides what the backend reports; reserve_tokens defaults
    # to a fifth of it, between 8k and 64k.
    keep_turns: 1
    keep_tokens: 40000
    anchor_messages: 1
    # The judge is OPT-IN and off by default: enabling it sends conversation
    # history to TypeSafe. Its key prefers the TYPESAFE_API_KEY env var.
    judge:
      enabled: false
      model: jev-latest
      base_url: https://api.typesafe.ai
      api_key: ""
      prune_threshold: 0.75
      max_blocks_per_call: 64

tools:
  run_command: true
  web_fetch: true
  tasks: true
  parallel_agents: true

security:
  approve_tools: false
  path_isolation: false
  env_isolation: false
  deny_elevation: true

reasoning:
  effort: ""
  thinking: true
  budget: 0

discovery:
  project_context: true
  skills: true
  trust_skills: false
  trust_hooks: false

ui:
  rendering_mode: tui
  group_tools: true
  show_thinking: true
  prompt_placeholder: "Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\\ forces it."
  transparent_blocks: true
  cron_list_json: false

sources:
  skill_dirs: []
  mcp: {}

hooks: []

sandbox:
  default: ""
  vm_name: ""
  port: 2226
  ssh_key_path: ~/.ssh/id_ed25519
  root: ""
  auto_snapshot: false
  targets: {}

remote:
  default: ""
  user: ""
  port: 0
  ssh_key_path: ""
  root: ""
  targets: {}
```

## Context compaction

A long session is measured against the backend's context window and compacted at
the ratio it has crossed, instead of at one absolute token count. Two tiers, the
second a superset of the first:

| Tier | Crossed at | What it does | Model calls |
|---|---|---|---|
| soft | `soft_ratio` (0.65) × usable window | Tombstones oversized old tool results — deterministic, no model call | 0 |
| smart | `smart_ratio` (0.80) × usable window | Classifies each history block and folds it into one `[state ledger]` message, keeping, pruning or folding; forces the fold when the gentle one would not land | 1 judge + 1 ledger |

Whether a pass *forces* — folding the whole history rather than the blocks the
judge left in place — is not a third threshold. It is derived: the pass forces
when the gentle fold it just built would not leave the conversation under its
trigger. Asking that question directly is more precise than a second ratio, which
had to serve every window size at once, and it is safe because forcing only ever
moves a block from kept to folded: a prune still needs the judge's probability and
confidence, so forcing costs verbatim fidelity and never a fact.

The *usable window* is the backend's context window less `reserve_tokens` — the
runway the model needs to finish its own answer — so the top rung still leaves the
reserve plus a fifth of the usable window, instead of a fifth of the raw one.
That reserve is an engineering hypothesis rather than a measurement: it defaults
to a fifth of the window between 8k and 64k, and a session that knows what its
model needs should set `reserve_tokens`. A backend that reports no window at all
can be given one with `window_tokens`.

The verbatim tail is sized by `keep_tokens` (40k by default), a budget rather
than a count of messages, so two heavy reads can no longer pin the window open.
`keep_turns` is the floor underneath it — how few messages the tail may ever
shrink to, one by default, the live turn alone — and the first
`anchor_messages` messages, the original task, are never rewritten, summarized or
pruned, so the goal cannot be compacted away. The ledger is rebuilt, and it is
rebuilt by *merging*: a later pass folds new facts into the existing one line by
line, so a summarizer that restates what the ledger already holds adds nothing to
it. Never a summary of a summary — the ledger is only ever shown to a call that
also carries turns no earlier pass compressed. Once the body outgrows one summary
(2000 tokens, the same ceiling the summarizer writes under) the next pass
*consolidates* it: one rewritten block instead of an addition, accepted only if
it still names every identifier the old body named, otherwise the merge stands.
`limits.compact_at` still speaks last (an absolute ceiling when set, `0` disables
compaction), and a backend that reports no context window falls back to it.

If a provider refuses a request for length anyway — the ladder is measured
against an estimate, so it can — bulle compacts once and sends the turn again.
That retry forces the fold, it happens at most once per turn, and it is
skipped entirely when `compact_at` is `0`.

The judge is **opt-in and off by default**: turning on
`limits.compaction.judge` sends conversation history — which can include source
code and secrets — to TypeSafe's System One model for classification, so it is
the one setting here that leaves the machine. Its key prefers the
`TYPESAFE_API_KEY` environment variable over `limits.compaction.judge.api_key`.
With the judge off, compaction behaves exactly as it did before the ladder
existed. The status line shows the live load and tier against the usable window
(`↕120k/160k · 0.75 · soft`), and `/status` reports the ledger and the last
pass's tier.

## Sandboxes & remote hosts

bulle runs on the host and sends every tool call to the target over SSH, so the
provider keys and the local filesystem never enter the environment the model
reaches. Two commands cover the two kinds of target:

```sh
# Local boite microVMs
bulle sandbox list              # configured sandbox.targets and running boite VMs
bulle sandbox pingu             # interactive session inside a VM
bulle sandbox pingu --snapshot  # snapshot the overlay disk when the session ends

# SSH hosts
bulle remote list               # hosts configured under remote.targets
bulle remote staging            # interactive session on a configured host
bulle remote deploy@build:2222 "run the tests"
```

`sandbox` targets resolve to local [boite](https://github.com/FacileStudio/boite)
microVMs: an entry in `sandbox.targets`, then a registered instance of that
name. `remote` targets resolve to a `remote.targets` entry, a direct
`user@host:port` address, or an `~/.ssh/config` host alias. Each reads its
defaults — user, port, identity, workspace — from its own group in `~/.bulle.yml`.

Scheduled jobs are not configured here anymore: one YAML file per job under
`~/.bulle/jobs/`, trusted with `bulle cron trust <name>` before it runs.
Use `bulle cron list` to view all jobs along with their enabled and crontab installation status.

## herdr

Run inside [herdr](https://herdr.dev), `bulle` reports its live state and
session identity over herdr's socket API (`internal/herdr/`). A bulle pane
shows as an agent with an idle / working / blocked state, and herdr holds a
reference to the run's transcript. This needs no herdr binary update and works
on any machine, including stock herdr.

After a herdr **server restart**, herdr restores a bulle pane as a plain
shell in its saved directory — bulle is not in herdr's compiled-in resume
table, and no config or plugin adds it. Reopen the session in that directory
with `bulle` (auto-resumes the newest session by cwd) or
`bulle --resume <transcript-path>`. Real auto-restore awaits herdr adding
bulle to its resume table; `--resume` already accepts the exact absolute
transcript path the reporter reports.

## Editors

`bulle --ide` (or `BULLE_IDE=1`) publishes the session to an editor over a unix
socket, so a plugin can watch a run and drive it: mark the lines bulle changes,
put an approval in front of you, run a prompt with your cursor's place
attached, and stop a run. It is off by default, and a session that publishes to
nobody creates no socket, no file and no goroutine.

The contract is [docs/ide-protocol.md](docs/ide-protocol.md); the first client
is [bulle.nvim](https://github.com/FacileStudio/bulle.nvim).

## Structure

```
main.go             Entrypoint: calls cmd.Execute
cmd/                CLI commands, Cobra tree, flag parsing, and Fang styling
internal/agent/     Agent lifecycle, tools wiring, headless mode, banner, flags
internal/approval/  Interactive and batch tool call approvals
internal/cost/      Token usage and cost calculation
internal/diff/      Syntax-highlighted unified diff generator
internal/history/   Command history and navigation
internal/layout/    Terminal dimensions and line truncation
internal/menu/      Autocompletion menu for commands and skills
internal/queue/     Input queueing during active turns
internal/sessions/  Session persistence, listing, rotation, resume
internal/settings/  CLI flags, ~/.bulle.yml, and environment configuration
internal/skills/    Agent skills discovery and execution
internal/status/    Spinner and progress status indicator
internal/tasks/     Task planning tool, validation, step updates
internal/theme/     Terminal color palettes and syntax themes
internal/thinking/  Collapsible reasoning viewport
internal/toolview/  Compact and grouped tool rendering
internal/compaction/ Zone/ledger context strategy: Plan, Apply, Tombstone, the judge
internal/jev/       TypeSafe System One client for the opt-in compaction judge
internal/tui/       Bubble Tea v2 model, key handling, rendering, slash commands
internal/usage/     Token accounting and context window headroom
internal/herdr/     Reports agent state and session identity to herdr over its socket API
internal/ide/       Publishes a session to an editor over a unix socket
```

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
