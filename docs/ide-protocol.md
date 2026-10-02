# bulle IDE protocol

The contract between a running `bulle` session and an editor plugin (first
consumer: `bulle.nvim`). A session that nobody is watching pays nothing: the
whole thing is off unless `--ide` is passed or `$BULLE_IDE` is set.

This file is the frozen interface. Both sides implement it as written.

## Discovery

While `bulle` runs with the IDE surface on, it writes one file per process:

```
~/.bulle/ide/<pid>.json
```

```json
{
  "v": 1,
  "pid": 12345,
  "root": "/home/yann/Code/Facile/bulle",
  "socket": "/run/user/1000/bulle/12345.sock",
  "session": "/home/yann/.bulle/sessions/20260924T160000Z-12345.jsonl",
  "started": "2026-09-24T16:00:00Z",
  "version": "0.76.0"
}
```

The file is created `0600` with its directory `0700`, and removed when the
process exits. A stale file left by a killed process is detected by checking
that `pid` is alive; a reader that finds a dead `pid` ignores the file and does
not delete it (the owner might still be starting up).

A plugin picks its session by matching `root` against its own working
directory, and falls back to the newest `started` when several match.

## Transport

A unix socket, same machine only. No TCP, no TLS, no ports.

- Default path `$XDG_RUNTIME_DIR/bulle/<pid>.sock`, falling back to
  `$TMPDIR/bulle-<uid>/<pid>.sock` when `XDG_RUNTIME_DIR` is unset.
- Newline-delimited JSON: one object per line, `\n` terminated, no literal
  newline inside an object.
- Every object carries `"v": 1` and a `"t"` naming its type.

**Forward compatibility is mandatory on both sides.** A receiver ignores an
unknown `t`, and ignores unknown fields inside a known `t`. Anything else makes
a version bump a breaking change, which it must not be.

A receiver that reads a `v` it does not support replies
`{"v":1,"t":"error","reason":"unsupported protocol version"}` and closes.

## Session to editor

| `t` | Fields | Meaning |
|---|---|---|
| `hello` | `pid`, `root`, `session`, `model`, `version` | First line after a client connects. |
| `turn` | `n` | A model turn started. `n` counts from 1, within the run. |
| `tool` | `id`, `name`, `status` (`start`/`done`), `ok`, `path` | A tool call started or finished. `path` only for file tools; `ok` only on `done`, since whether a call worked is unknowable before it runs. |
| `edit` | `id`, `path`, `tool`, `first`, `last`, `added`, `removed`, `diff` | A file changed. `first`/`last` are 1-based inclusive line numbers in the **new** file. `diff` is optional and bulle does not currently send it, so a receiver must not depend on it. |
| `approval` | `id`, `tool`, `input` | A tool is waiting for the user's yes or no. `input` is the raw tool JSON, verbatim, so the editor can show exactly what is about to run. |
| `done` | `reason`, `cost` | The run finished. `reason` is `end_turn`, `max_iterations`, `cancelled` or `error`. `cost` is US dollars, zero when the backend reported none. |
| `error` | `reason` | Protocol level problem. The connection closes after it. |

`edit` is the important one. It carries the line span so the editor can mark it
without re-deriving anything, and `diff` so it can show what changed without
reading the file.

## Editor to session

| `t` | Fields | Meaning |
|---|---|---|
| `hello` | `root`, `pid` | First line after connecting. |
| `send` | `text`, `path`, `line`, `branch` | Run `text` as a prompt, with the editor context attached. |
| `open` | `path`, `line` | Ask bulle to scroll its own view to that place. |
| `approve` | `id`, `allow` | Answer an `approval`. `allow` false is a refusal. |
| `stop` | | Cancel the current run. |

An `approve` naming an `id` that is not waiting is ignored, not an error: a
client that clicked late should not break the session.

## Approval is an approval surface

Anything that renders `approval` is a security boundary:

- It shows `input` exactly as received. It does not summarise, truncate silently,
  or reformat it.
- It fails closed. A lost connection, a malformed reply, or a timeout is a
  refusal, never an implicit yes. The session's own timeout decides, and it
  denies.
- It never answers on the user's behalf. No default yes, no "remember this
  choice" that widens scope.

An `approve` allows one call. Nothing in this protocol widens it to a session,
so a receiver that offers "always allow" is deciding that itself, outside the
wire.

**No editor attached is not a refusal.** A session that has published a socket
nobody has dialled asks nobody, and its own approval surface — for `bulle`, the
terminal prompt — decides the call. A client must therefore not read a missing
`approval` as "the session has no approvals": it is asked only about the calls
that arrive while it is attached.

## Why not MCP

bulle consumes MCP; the editor is a second client of the same session, and the
session lives in bulle's process. An IDE socket is a session attachment, not a
tool the model can call.
