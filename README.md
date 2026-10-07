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

limits:
  max_iterations: 5
  max_concurrency: 16
  # compact_at: 75000
  compaction:
    judge:
      enabled: false
      provider: jev

security:
  deny_elevation: true
  env_isolation: false

editor:
  # editor: /usr/bin/vim
  # prompt_edit_key: ctrl+g

sources:
  skill_dirs: []
  mcp: {}
```

The block above is the full scaffold — the file bulle writes on first boot,
and the `TestExampleConfigIsTheScaffoldTemplate` test checks it is byte-identical
to `example.bulle.yml`. The rest of the surface (tools, reasoning, discovery,
ui, sandbox, remote, chat, gates, sources, hooks) is documented in
[docs/configuration.md](docs/configuration.md); a key missing from the block
above is not a broken setting, it is a setting kept to its default.

## Context compaction

Two tiers: soft (deterministic, no model call) and smart (a judge pass
that classifies each history block and keeps, prunes or folds it).
Only four keys are the user's decisions; the ratios and tail bounds
are internalised defaults — read when present, not documented or
scaffolded:

| Key | Default | What it does |
|---|---|---|
| `limits.compact_at` | unset (backend's window or 75000) | Absolute ceiling; `0` disables compaction |
| `limits.compaction.judge.enabled` | `false` | Opt-in. Sends history to the judge's provider |
| `limits.compaction.judge.provider` | `jev` | `jev` (TypeSafe System One) or `clef` (Cloudflare via OpenRouter) |
| `limits.compaction.window_tokens` | unset | Override for gateways that under-report the window |

The judge is **opt-in and off by default**: turning on
`limits.compaction.judge` sends conversation history — which can include source
code and secrets — to the judge's provider for classification.
`/status` reports the provider and model version that actually answered the
last call.

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
