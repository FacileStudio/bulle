# Sudo, the agent, and your password

This document answers three questions in order:

1. Can bulle block elevation commands in `run_command` through `~/.bulle.yml`?
2. Is blocking elevation a good idea?
3. If elevation is allowed, how do we let the human type a password without the
   agent ever seeing it?

## How run_command works today

`run_command` is implemented in the nacelle SDK, in `tools/command.go`.
It runs the command as:

```go
exec.CommandContext(ctx, "/bin/sh", "-c", command)
```

Consequences that matter here:

- The command runs with **this process's own privileges**: whatever user
  launched bulle, that is who the agent runs as.
- There is **no TTY**. Interactive `sudo` asks for a password on a terminal,
  and there is none, so plain `sudo apt install foo` fails today with
  "sudo: a terminal is required".
- `sudo -S` reads the password from **stdin** instead. stdin is not a
  terminal either, but a command like `echomypassword | sudo -S ...` or
  `sudo -S ... < somefile` is technically possible. That is the hole.

The SDK already has a scanner for shell escapes,
`tools/command_escape.go` (`checkCommandEscapes`). It walks the command,
splits on `;`, `&&`, `||`, `|`, `&`, and refuses a list of shell builtins
when strict confinement (`security.path_isolation`) is on. A bulle rule has
a natural home right next to it.

## Question 1: can we configure this in ~/.bulle.yml?

Not today: there is no setting for it. Two ways to get there:

### Option A: a config key (recommended)

Add to the existing `security:` group:

```yaml
security:
  deny_sudo: true   # default: true
```

When it is on, `run_command` refuses any command that attempts elevation,
using the same walk the escape scanner already does:

- `sudo`, `sudo -S`, `sudo -k`, any sudo flag
- `doas`, `pkexec`, `su`, `su -c`, `su root`
- after every chain separator, not just at the start: `go build; sudo make install`
- obfuscated forms: `\sudo`, `env sudo`, `sh -c 'sudo ...'`, `$(sudo ...)`,
  `` `sudo ...` ``

### Decided (2026-09-12): shipped as `security.deny_elevation`

Option A shipped as `deny_elevation` — broader than the original `deny_sudo`
sketch: it refuses every elevation primitive (`sudo`, `doas`, `su`, `pkexec`,
`docker`, `chroot` and relatives, however spelled, quoted, chained or behind a
path) and refuses to run any setuid-root binary. Default on. Settings wiring in
bulle is `internal/settings/config.go` (`DenyElevation`, env
`BULLE_DENY_ELEVATION`); detection lives in the SDK at
`tools/command_escape.go` (`checkCommandElevation`), checked before execution.

The denial message carries the protocol: the blocked agent is told not to
retry, to report the blocker in its result and finish — the fix for a parallel
subagent sitting stuck on a permission it will never get.

Two escape hatches exist on purpose: `deny_elevation: false` in config, and
parallel fan-outs can be stopped — `/parallel cancel [batch]` for the human,
the `parallel_cancel` tool for the model, both killing the fan-out's context so
still-running tasks report back as cancelled instead of grinding forever.

### Option B: a hook (exists today, zero code)

The hook system can already veto a tool call: a hook that exits 2 denies the
call and the model reads stderr as the reason.

```yaml
hooks:
  - on: before_tool_call
    match: [run_command]
    run: ~/bin/deny-sudo.sh
```

with `~/bin/deny-sudo.sh` reading the event JSON on stdin and grepping the
command. It works, but every user has to write the script, and the hook runs
a whole extra process per command. Option A is the same idea built in.

## Question 2: is blocking sudo a good idea?

Yes, with one honest caveat.

**Why yes:**

- An agent that can run arbitrary shell is already effectively you. Giving it
  root too removes the last natural stopping point.
- The realistic threat is not the model being evil. It is a prompt injection:
  a README, an install script, an issue comment, or a file the agent read says
  "run `sudo curl ... | sh`" and a hard-working model complies. A deny rule
  stops that regardless of where the instruction came from.
- `sudo -S` with a password on stdin is exactly the credential-leak shape the
  next section is about. Blocking it closes the hole.

**The caveat — what this does NOT do:**

A string check on the command is a *policy guard*, not a *security boundary*.
It stops the model from attempting elevation. It does not stop:

- calling setuid binaries other than sudo
- `docker run -u root` if docker is available
- anything already running as root on the machine

The web research is blunt about this. Claude Code has `Bash(sudo:*)` deny
rules and still documents that Bash matching is "best-effort, not a hardened
shell sandbox" (code.claude.com/docs/en/permissions). Denylists have been
bypassed in practice: Claude Code silently skipped deny rules past 50 chained
subcommands (disclosed April 2026), deny rules failed to match
`gcloud -h` (anthropics/claude-code#25621), and Trail of Bits reached RCE
through argument injection into an *allowed* command. The consistent advice
across sources: denylist for the obvious cases, human approval for the rest,
and OS-level enforcement as the real boundary.

So the honest framing for the docs: `deny_sudo` catches accidents, lazy
models and injected instructions. If you need a guarantee, run bulle under
an account that is not in the `sudo` group, or rely on `security.env_isolation`
and path isolation plus OS sandboxing. String matching is the first rung, not
the wall.

## Question 3: typing the password safely

The rule: **the password must never exist as text the agent can see, request,
or influence.** It must never pass through the model's context, the
transcript, a config file the agent can read, or an env var the agent can echo.

Concretely, ranked:

### Do not: pipe the password from config or env

The harness must never hold the password and feed it to `sudo -S`'s stdin.
That puts the credential one `echo`, one `printenv`, or one prompt injection
away from the transcript, and it teaches the model that sudo is just available.
Never store a sudo password in `~/.bulle.yml`, tiroir, or the environment
for the agent's use.

### Best: NOPASSWD per command in sudoers

If a workflow legitimately needs elevation (restarting a service, mounting a
disk), configure sudoers for those exact commands:

```
# /etc/sudoers.d/bulle
yann ALL=(root) NOPASSWD: /usr/bin/systemctl restart myservice, /usr/bin/mount
```

Sudoers is the tool designed for "these exact commands, no password". It is
auditable with `sudo -l`, it cannot be widened by editing the agent's config,
and it survives obfuscated shell because sudo itself does the matching. The
agent runs `sudo systemctl restart myservice`, sudo does not ask, no password
exists anywhere.

### Good: the human types, in their own terminal

When a command needs elevation and sudoers does not cover it, bulle pauses
and asks you to run that one command yourself in your own terminal. The agent
sees only "the human ran it, exit code 0". No password ever crosses the
process boundary.

### Good: harness approves, sudo prompts on a fresh pty

When a command needs elevation and sudoers does not cover it, bulle can
run the elevated command itself by letting **sudo's own prompt** collect the
password on a pty the harness allocates but never reads. The harness writes
the command to the pty, forwards only the output after the prompt and the
exit code, and never buffers input. The model sees, at most, "elevated
command approved by the human, exit code N".

This is the correct default for the "the agent should finish the job" case,
ranked just under sudoers NOPASSWD. It is phase 2: not in the first
`deny_elevation` release.

Two hard requirements make it safe:

- **A fresh pty per elevated command, closed after.** sudo caches the
  credential for ~15 minutes per tty. Reusing one pty across commands
  turns the first password into a 15-minute unlock for everything the
  model runs, including commands nobody approved. A new tty per command
  gives each elevation its own timestamp domain. Belt and suspenders:
  run `sudo -k` after the command too.
- **The approval dialog shows the exact command.** The human types the
  password into a prompt that names what is being run. An approval of an
  unread command is worse than no prompt.

The interaction with `deny_elevation`: the deny fires first. An explicit
human approval of that specific command is what routes it to the pty path.
One decision, not two.

### What the agent sees in every case

The agent never sees a password. It sees, at most:

- `"<token>" attempts privilege elevation and is blocked (security.deny_elevation)` with the report-and-finish protocol appended
- `elevated command approved by the human, exit code 0`

## Summary

| Question | Answer |
|---|---|
| Configure a sudo block in `~/.bulle.yml` | Yes: shipped as `security.deny_elevation`, default on, all elevation primitives plus setuid-root binaries |
| Good idea? | Yes, as a policy guard. Not a security boundary; OS-level enforcement is the wall |
| Safe password entry | Never through the agent. Sudoers NOPASSWD per command, the human runs the command themselves, or (phase 2) a fresh pty per elevated command whose prompt the harness never reads |
