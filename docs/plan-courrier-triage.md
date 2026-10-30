# Plan: courrier triage (API + CLI)

Status: approved scope, ready to build. Written 2026-10-30 as a cold-start handoff for an
agent working in the `Courrier` and `courrier-cli` repos, with no memory of the conversation
that produced it. Parent plan: `bulle/docs/plan-true-assistant.md` (work package CX).

## Goal

Courrier tells its clients which mail is bulk and which comes from a person Yann knows, so
no client has to guess from sender names. One CLI call returns the unread mail of every
account, already classified:

```
courrier triage --all --unread --json
```

Bulle's assistant calls this instead of looping over accounts, reading 500 subjects and
asking a free model to ignore marketing.

## Why (evidence)

Checked against the code on 2026-10-30:

- `apps/api/modules/mail/imap.go` fetches envelopes plus one header, `References`
  (`referencesSection`). No `List-Unsubscribe`, `List-Id`, `Precedence` or `Auto-Submitted`
  is read, so bulk mail cannot be told from human mail.
- `apps/api/schemas/email.go` has no bulk or category column. `courrier inbox --unread --json`
  returns Railway, OUIGO and BNP rows next to real people.
- `SyncFolderEmails` (`service.go:197`) caches at most the newest 100 envelopes per folder
  (`fetchEnvelopes(client, folder.Path, 100)`). A "people I have written to" list built from
  the cached Sent folder would cover only the last 100 sent messages per account.
- The CLI has no cross-account call. An agent must run `courrier accounts` and then one
  `courrier inbox` per account (3 accounts today: ids 1, 2, 3).
- The Sent folder is already discoverable: `courrier folders --json` carries
  `"type": "sent"`, and `courrier list sent --json` works. No change needed there.

## Design

### Classification (decided in the API, at read time from stored columns)

Each conversation row gets one `class` and one `reason`:

| class | meaning | reasons (first match wins) |
|---|---|---|
| `bulk` | automated or marketing | `list-unsubscribe`, `list-id`, `precedence-bulk`, `auto-submitted`, `noreply-sender` |
| `known` | a person Yann has dealt with | `in-sent-thread` (the thread holds a message Yann sent), `sent-to` (sender is in contacts) |
| `unknown_human` | looks personal, no history | `no-signal` |

Order: bulk signals first, then known, else unknown_human. `noreply-sender` matches the local
part against `noreply`, `no-reply`, `donotreply`, `do-not-reply`, `notifications`,
`notify`, `mailer-daemon`, `postmaster`, `newsletter`, `news`, `marketing`, `updates`.
This is a fallback: header signals win when present.

### Data

1. `schemas.Email` gains `Category string` (`""`, `bulk`, `auto`) and `BulkReason string`,
   filled at sync from the headers `List-Unsubscribe`, `List-Id`, `Precedence`
   (`bulk`, `list`, `junk`), `Auto-Submitted` (anything but `no`), `Feedback-ID`,
   `X-Auto-Response-Suppress`. The header fetch is the existing `referencesSection` widened to
   all these fields in the same FETCH, so sync cost does not change.
2. `schemas.Contact` (new table): `ID`, `AccountID`, `SpaceID`, `Email` (lowercased), `Name`,
   `SentCount`, `LastSentAt`. Unique on `(account_id, email)`. Filled from the To and Cc of
   every message cached in a `sent` folder, upserted on each sync, and on every successful
   `send`. Contacts are scoped to the user across accounts: an address Yann wrote from any
   account is known on all of them.
3. Backfill, run once after the migration:
   - Emails: for cached rows in `inbox` folders with `Category` unset and a date within the
     last 60 days, fetch the header block by IMAP UID in batches of 50 and set the columns.
     Rows older than 60 days keep the sender-pattern fallback.
   - Contacts: a deep sync of each account's `sent` folder, up to the newest 1,000 envelopes,
     reading only envelopes (no bodies), to fill `Contact`. Normal sync keeps its 100 cap.
   - Both are idempotent and resumable, triggered by the next `sync` after deploy and by
     `POST /api/accounts/{id}/mail/backfill`.

### API

- `GET /api/mail/triage` across all the user's accounts (and spaces' shared accounts, as
  `GET /api/accounts` already does). Query: `unread=true`, `class=known,unknown_human`
  (comma list, default all), `since=<RFC3339 | 2h | 1d>`, `limit` (max 100, default 50),
  `page`, `account=<id>` (optional filter).
- `GET /api/accounts/{id}/mail/contacts` and `GET /api/mail/contacts` (all accounts): rows
  `{email, name, sent_count, last_sent_at}` ordered by `last_sent_at` desc, `limit`, `page`.
- Add `category`, `bulk_reason` to the existing email JSON (`EmailResponse`) so every client
  can see them. Additive, no break.
- Response of triage:

```json
{
  "generated_at": "2026-10-30T10:00:00Z",
  "total": 18,
  "counts": {"known": 2, "unknown_human": 1, "bulk": 15},
  "items": [
    {
      "account_id": 1, "account": "yann@facile.studio",
      "thread_id": "<message-id>", "email_id": 1880,
      "subject": "Railway security update", "from_address": "hello@notify.railway.app",
      "from_name": "Railway", "date": "2026-10-23T20:41:41Z",
      "message_count": 1, "unread_count": 1,
      "class": "bulk", "reason": "list-unsubscribe"
    }
  ],
  "page": 1, "limit": 50
}
```

`counts` always covers the whole filtered set, not only the page, so a client can say
"15 automated mails ignored" without fetching them.

### CLI (`courrier-cli`)

- `courrier triage [--account N | --all] [--unread] [--class known,unknown_human,bulk]
  [--since 2h] [--limit 50] [--page 1] [--json]`. `--all` is the default when no account is
  given and no default account is set; with a default account set, `--all` must be explicit.
  Human output: one line per conversation, class tag, sender, subject, age; footer with the
  counts.
- `courrier contacts [--account N | --all] [--limit 50] [--json]`.
- `courrier inbox` and `courrier list` gain `--since`, same grammar as triage.
- JSON output is exactly the API body. Exit code 0 on success; a 404 from an older API
  prints "this Courrier instance does not support triage; upgrade the server" and exits 1,
  which is how bulle detects a missing feature.
- Update `integrations/SKILL.md` and `README.md`: document `triage` and `contacts`, and tell
  agents to prefer `triage` over looping `inbox` per account.

## Work packages

| WP | Repo | Files (new or edited) | Depends on |
|---|---|---|---|
| CX1 headers and category | Courrier | `apps/api/modules/mail/imap.go` (widen header section, parse), `apps/api/modules/mail/classify.go` (new: header and sender rules), `apps/api/schemas/email.go` (columns), `apps/api/modules/mail/service.go` (store at sync), `apps/api/modules/mail/types.go` and `router.go` (expose fields) | none |
| CX2 contacts | Courrier | `apps/api/schemas/contact.go` (new), `apps/api/schemas/migrate.go` (add to AutoMigrate), `apps/api/modules/mail/contacts.go` (new: upsert from sent, list), `apps/api/modules/mail/service.go` (call upsert from sync and send) | none |
| CX3 triage and backfill | Courrier | `apps/api/modules/mail/triage.go` (new: class resolution, counts, since parsing), `apps/api/modules/mail/backfill.go` (new), `router.go` (routes), `documentation.go` (API reference), `docs/api.md` | CX1, CX2 |
| CX4 CLI | courrier-cli | `cmd/triage.go`, `cmd/contacts.go` (new), `cmd/list.go` (`--since`), `internal/client/client.go` and `types.go` (calls and types), `integrations/SKILL.md`, `README.md`, `CHANGELOG.md` | CX3 (a mock server lets it start earlier) |

CX1 and CX2 run in parallel in separate worktrees; both touch `service.go`, so they edit
different functions and the integrator resolves the small merge. CX4 can start against a
fixture server built from the JSON contract above.

## Tests

- `classify_test.go`: table test, one row per signal (each header, each noreply pattern, a
  plain human mail, a human mail with `Precedence: list` as bulk, `Auto-Submitted: no` as
  not bulk). Use real-shaped headers from Railway, OUIGO and a person.
- `contacts_test.go`: upsert counts, lowercase, cross-account scope, To plus Cc.
- `triage_test.go`: class order (a sent-thread reply with a noreply sender is bulk, because
  bulk wins), counts cover the whole set, `since` grammar, pagination.
- `backfill_test.go`: idempotent, skips rows already set, stops at 60 days.
- Database tests need Postgres (`TEST_DATABASE_URL`); without it they skip loudly. Run them
  with it set before declaring done.
- CLI: `client_test.go` against `httptest`, `triage` JSON golden file, a 404 yields the
  upgrade message and exit 1.

## Repo rules

- Courrier: Go 1.24, vendored modules (`go mod vendor` after dependency changes), migrations
  run on startup through `schemas.Migrate`, every API route lives under `/api`, `filet check`
  must pass (config is the Go-family standard in `filet.yml`), read `CLAUDE.md`. Document new
  routes in `documentation.go` and `docs/api.md`.
- courrier-cli: follows `CLI-STANDARD` as `CLAUDE.md` states, no sixth dependency, `mise run
  check` is the gate, JSON mode prints one document and nothing else.
- No inline comments, no comments inside function bodies; doc comments on exported
  declarations only. Conventional Commits, lowercase, no emoji, no attribution lines.
- Do not log mail subjects or addresses at info level.

## Release

- Courrier API: deploys through the repo's Docker flow. The repo has no release tags today
  (only a backup branch), so the builder asks Yann how the server is deployed before
  shipping and does not invent a tag scheme.
- courrier-cli: `v0.3.0` to `v0.4.0` (new commands, additive), via the repo's goreleaser
  flow and `/ship`.
- Order: API deployed first, then the CLI release, then bulle (`v0.86.0`) switches its mail
  source to `triage`. Bulle keeps a local fallback for an older courrier.

## Acceptance

1. `courrier triage --all --unread --json` returns one document covering accounts 1, 2 and
   3, and every Railway, OUIGO, BNP, LinkedIn, Spedidam and GermanPod101 row has
   `class: bulk`.
2. Flavie-Marie FUCHS and Nathalie Lenzeele rows are `known` if Yann ever wrote to them, else
   `unknown_human`, and never `bulk`.
3. `counts` matches the number of rows when paging through the full set.
4. A second sync adds no duplicate contacts and no extra header fetches for classified rows.
5. `courrier contacts --all --json` lists people Yann wrote to more than 100 messages ago.
6. Against an old server, `courrier triage` exits 1 with the upgrade message.

## Risks

- Header backfill talks to IMAP for up to 60 days of inbox mail per account. Batch, throttle,
  and keep it resumable so a slow server does not block normal sync.
- A mailing list that Yann replied to lands in `in-sent-thread`; bulk signals win, so it
  stays bulk. A real person sending through a list header would be classed bulk. Accepted:
  the digest shows the bulk count, and Yann can add a VIP in bulle to override.
- Contacts derived from Sent include one-time recipients (a shop, a landlord). They count as
  known. Acceptable; `unknown_human` still reaches the digest.
