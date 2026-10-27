# Plan: Personal Assistant & Matrix Automation

Status: proposed, ready for review.
Date: 2026-10-27
Author: saravenpi

## 1. Goal

Turn `bulle` into a proactive, high-signal personal assistant accessible via Matrix (and any Matrix client such as Element, Clawd, or Hermes).

The assistant must:
1. Run background pulses every 30 minutes over `freellmapi` (zero API cost) to evaluate emails (`courrier`), tasks (`opus`), and calendar events (`agenda`).
2. Enforce a strict "nudge budget": remain completely silent when nothing is urgent, alerting on Matrix only when actionable events occur.
3. Conduct scheduled proactive check-ins (midday and evening) to review priorities, capture daily debriefs, and record entries directly into `~/Documents/NuagePersonal/Wiki/Journal/YYYY-MM-DD.md`.
4. Hold a persistent conversation in Matrix so the user can reply naturally without losing context.

---

## 2. Current State & Gap Analysis

A diagnostic across the system and codebase identified the following reality:

| Component | Current State | Gap / Blocker |
|---|---|---|
| **Local LLM** | `freellmapi` is running in Docker at `http://localhost:3001/v1`. Profile `free` is defined in `~/.bulle/profiles/free.yml`. | None. Verified working via curl with `CUSTOM_API_KEY` from `tiroir`. |
| **Facile Tools** | `courrier`, `opus`, and `agenda` CLIs are installed and working. | None. Tools can be called from bash commands by the agent. |
| **Journaling** | `~/Documents/NuagePersonal/Wiki/Journal/` exists with daily `YYYY-MM-DD.md` notes. | Needs explicit prompt steering so the assistant appends debrief notes under `## Journal`. |
| **Chat Daemon** | `internal/chat` has full pure-Go Olm/Megolm Matrix E2EE support. | Daemon collision: an old `kori-chat.service` process (PID 3286839) is running in memory and holding the Matrix `/sync` connection. |
| **Cron Delivery** | `bulle cron` runs headless jobs on schedule. | **Hard blocker**: `validateDelivery` in `internal/agent/cronlog.go:25` strictly allows `file:<dir>` and `""`. Matrix delivery is rejected before running. |
| **IPC** | No IPC socket exists for `bulle chat`. | Outside processes cannot ask the running chat daemon to send a message or run a turn. |

---

## 3. Best Practices & Design Principles (Dot, Muse & Proactive Agents)

Drawing on production architecture patterns from human-centric personal assistants (such as New Computer's **Dot** and **Muse**), the system implements five core principles:

### 3.1 The Nudge Budget (Silence Over Spam)
Pinging the user every 30 minutes with "No new email, 28 tasks left" creates cognitive fatigue and alert blindness.
- The 30-minute pulse is an evaluation check, not an automatic notification.
- **Gating threshold**: the agent speaks on Matrix only if there is an unread email from an actual person (not automated newsletters), a task due today, or a calendar event within the next 60 minutes.
- If nothing requires attention, it writes a quiet line to a local log and exits silently.

### 3.2 Predictable Grounding Anchors
Instead of erratic interruptions throughout the day, the assistant anchors interactions around predictable temporal routines:
- **09:00 Morning Kickoff**: Top 1-2 priorities, day agenda, clear daily focus.
- **14:00 Midday Check**: Brief priority check and progress pulse.
- **20:00 Evening Retrospective & Daily Debrief**: Reviews the day's progress and focus. When the user responds, summarizes the conversation directly into `~/Documents/NuagePersonal/Wiki/Journal/YYYY-MM-DD.md`.

### 3.3 Pre-Structured Memory Synthesis (The Dot Pattern)
Dot demonstrated that dumping raw conversation logs into vector embeddings leads to noisy, hallucinated recall. Instead, memories and reflections must be evaluated and structured *before* storage:
- When debriefing with the user, the assistant extracts discrete fields:
  - **Focus**: what was accomplished or attempted today.
  - **Friction**: any blocker, delay, or source of stress.
  - **Anchor**: the primary priority or next right action for tomorrow.
- Entries are saved with ISO-8601 timestamps and clean Markdown headers under `## Journal` in `~/Documents/NuagePersonal/Wiki/Journal/YYYY-MM-DD.md`.

### 3.4 Layered Memory: Core Profile vs. Episodic Journal (The Muse Pattern)
A personal assistant must maintain clear separation between identity and daily logs:
- **Core Profile State**: deterministic facts about the user, work hours, communication preferences, and active key projects (stored in `~/.bulle.yml` or a core profile document).
- **Episodic Stream**: the timestamped daily journal entries capturing evolving day-to-day context.
- This prevents context window saturation and ensures the assistant maintains a stable persona across turns.

### 3.5 Friction-Free Scaffolding (Low Cognitive Load Interaction)
Standard AI chat drops users into an intimidating blank prompt. Dot proved that personal assistants excel by offering guided, low-friction interactions:
- When checking in, the assistant avoids overwhelming open-ended questions (e.g. "What do you want to work on today out of your 28 tasks?").
- Instead, it distills options into 1 or 2 concrete, actionable suggestions: *"Looking at Opus, task X is due today. Would you like to focus on that first, or review the new email from client Y?"*

### 3.6 Permission Tiers: Autonomous Read vs. Gated Write
- **Autonomous Read**: 30-minute pulses run unattended, gathering data with read-only CLI calls (`courrier inbox`, `opus task list`, `agenda events list`).
- **Gated Write**: Any state-mutating action (sending an email, marking a task done, scheduling a calendar invite) requires explicit confirmation in the Matrix chat.

### 3.7 Single Connection & IPC Architecture
A Matrix account allows only one `/sync` connection at a time. The running `bulle-chat.service` daemon owns this connection and the Megolm encryption keys in SQLite (`~/.bulle/chat/matrix.db`).
- Cron jobs must never open `matrix.db` directly to send messages, as SQLite concurrency and Olm ratchet state would desynchronize.
- Instead, `bulle chat` opens a local Unix domain socket at `~/.bulle/chat.sock`.
- Cron jobs and CLI commands dispatch outgoing messages and triggers across this socket.

---

## 4. Technical Architecture

```
+-------------------------------------------------------------+
| Cron Scheduler (crontab)                                     |
|                                                             |
|  [assistant-pulse] (every 30m)    [assistant-checkin] (20:00)|
|  - freellmapi (localhost:3001)    - freellmapi              |
|  - courrier, opus, agenda         - daily debrief check-in  |
|  - delivery: matrix               - delivery: matrix        |
+------------------------------+------------------------------+
                               |
                               v (dispatches JSON over Unix socket)
+-------------------------------------------------------------+
| Unix Socket: ~/.bulle/chat.sock                             |
+------------------------------+------------------------------+
                               |
                               v
+-------------------------------------------------------------+
| bulle-chat.service (Running Daemon)                         |
|                                                             |
|  - Matrix Client (mautrix-go + goolm pure Go E2EE)          |
|  - Room Registry (Persistent Session + Compaction)          |
|  - Dispatches message or triggers room turn                 |
+------------------------------+------------------------------+
                               |
              +----------------+----------------+
              |                                 |
              v                                 v
     Matrix Room (Encrypted)         Personal Journal Note
     !cEBaQ0n...                     ~/Documents/NuagePersonal/
     (User reads on Element/Clawd)   Wiki/Journal/YYYY-MM-DD.md
```

---

## 5. Implementation Steps

### Step 1: Daemon Migration
1. Stop and disable the old lingering process:
   ```sh
   systemctl --user stop kori-chat
   systemctl --user disable kori-chat
   ```
2. Enable and start the official daemon:
   ```sh
   bulle chat install
   ```
3. Verify running status:
   ```sh
   systemctl --user status bulle-chat.service
   ```

### Step 2: Bulle Chat IPC Server (`internal/chat/ipc.go`, `cmd/chat.go`)
1. Create `internal/chat/ipc.go`:
   - Bind Unix domain socket at `~/.bulle/chat.sock` with `0600` permissions.
   - Clean up socket file on startup if dead, and on process termination.
   - Support command payload:
     - `{"action": "send", "room": "<optional_room_id>", "text": "..."}`
     - `{"action": "trigger", "room": "<optional_room_id>", "prompt": "..."}`
2. Update `cmd/chat.go`:
   - Start IPC listener inside `runChatDaemon()`.
   - Route `send` to `adapter.Send()`.
   - Route `trigger` to `rooms.get()` and `room.answer()`.

### Step 3: CLI Subcommands (`cmd/chat_send.go`, `cmd/chat_trigger.go`)
1. Create `cmd/chat_send.go`:
   - `bulle chat send [--room <id>] <message>`
   - Connects to `~/.bulle/chat.sock`, sends payload, prints result.
2. Create `cmd/chat_trigger.go`:
   - `bulle chat trigger [--room <id>] <prompt>`
   - Prompts the active room session and sends reply to Matrix.

### Step 4: Cron Delivery to Matrix (`internal/agent/cronlog.go`)
1. Update `validateDelivery(delivery string) error`:
   - Accept `delivery == ""`
   - Accept `strings.HasPrefix(delivery, "file:")`
   - Accept `delivery == "matrix"` or `strings.HasPrefix(delivery, "matrix:")`
2. Update `appendCronLog()`:
   - When delivery is `matrix` or `matrix:<room>`, dial `~/.bulle/chat.sock` and dispatch `{action: "send", text: logText}`.
   - If daemon is unreachable, log warning to stderr and fallback to `file:~/.bulle/logs/delivery-fallback.log`.

### Step 5: Cron Job Profile Inheritance (`internal/settings/cron.go`, `internal/agent/cronapply.go`)
1. Add `Profile string yaml:"profile"` to `CronJob` struct in `internal/settings/cron.go`.
2. Update `applyJob()` in `internal/agent/cronapply.go`:
   - If `job.Profile != ""`, load profile from `~/.bulle/profiles/<profile>.yml` and merge provider settings before job overrides.
   - Allows clean jobs specifying only `profile: free`.

### Step 6: Assistant System Prompt & Configuration (`~/.bulle.yml`)
Configure `~/.bulle.yml`:
1. System prompt defines the executive assistant persona incorporating Dot & Muse patterns:
   - Clear, calm, constructive, practical.
   - Scaffolds decisions into 1-2 concrete next steps to minimize cognitive load.
   - Mentions available tools: `courrier`, `opus`, `agenda`.
   - Explains the Dot-style pre-structured journaling protocol: when the user shares daily debriefs, format and append entries with `Focus`, `Friction`, and `Anchor` under `## Journal` in `/home/yann/Documents/NuagePersonal/Wiki/Journal/YYYY-MM-DD.md`.
2. Ensure Matrix target room is explicitly set:
   ```yaml
   chat:
     matrix:
       rooms: ["!cEBaQ0n-UmUWdY8sKifTCvu0u7f0ogBonwSHmsCcEys"]
   ```

### Step 7: Job Definitions (`~/.bulle/jobs/`)
Link jobs from `/home/yann/.dotfiles/bulle/.bulle/jobs/`:

1. **`assistant-pulse.yml`**:
   ```yaml
   name: assistant-pulse
   when: "*/30 * * * *"
   profile: free
   commands: true
   delivery: matrix
   workdir: "~"
   prompt: |
     Check courrier inbox (account 1), opus task list, and agenda events list (calendar 9).
     Evaluate if anything requires urgent human attention:
     - New unread email from an actual person (ignore marketing/notifications).
     - Tasks due today.
     - Calendar events starting within the next 60 minutes.
     If nothing is urgent, do not output anything (exit completely silent).
     If there is something urgent, output a concise 2-3 bullet point summary with clear, direct phrasing and 1 suggested next action.
   ```

2. **`assistant-checkin.yml`**:
   ```yaml
   name: assistant-checkin
   when: "0 14,20 * * *"
   profile: free
   commands: true
   delivery: matrix
   workdir: "~"
   prompt: |
     Send a friendly, grounding check-in message to Yann.
     Ask how his day is progressing, what is top of mind, and invite him to do a quick daily debrief.
   ```

3. Trust and install jobs:
   ```sh
   bulle cron trust assistant-pulse
   bulle cron trust assistant-checkin
   bulle cron install assistant-pulse
   bulle cron install assistant-checkin
   ```

---

## 6. Verification & Quality Gates

1. **Code Quality**:
   - Run `filet check` across all modified Go files (must stay clean: 0 findings, functions under 35 lines, no inline or function-body comments).
2. **Compilation & Tests**:
   - `go test -tags=goolm ./internal/chat/... ./internal/agent/... ./cmd/...`
3. **IPC Socket Test**:
   - Run `bulle chat send "Test message from CLI"` and verify delivery into Element room.
4. **Local LLM Verification**:
   - Run `bulle cron run assistant-pulse` and verify it uses `http://localhost:3001/v1` via `freellmapi`.
5. **Journaling Test**:
   - Send a reply in Matrix debriefing the day and verify structured entry appended to `~/Documents/NuagePersonal/Wiki/Journal/YYYY-MM-DD.md`.
