// Package sessions logs and manages session transcripts.
package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

// SessionLog is the record of a session on disk: one JSONL file per run,
// under ~/.bulle/sessions/, holding the questions that were typed and the
// answers that came back.
//
// It holds a path and nothing else — no handle, no buffer, no goroutine. Each
// line is an open, an append and a close, which costs a syscall per line and
// buys the property that matters here: the file on disk is complete after
// every line, so a session killed with ctrl+c, a crashed terminal or a laptop
// closing lid leaves a readable transcript rather than a truncated one. A
// transcript that only survives a clean exit is a transcript you cannot rely
// on, and the write rate is one line per thing a human said or read.
//
// A nil SessionLog records nothing and every method tolerates it. That is the
// state a machine gets when the directory or the file could not be opened —
// a read-only home, a full disk, no resolvable HOME. say() calls line() on
// every transcript line without checking, deliberately: the alternative is a
// nil test at the one call site that runs most, guarding a feature that is
// beside the point of the program.
type SessionLog struct {
	path       string
	lastSize   int64
	writeError bool
	backend    string
	model      string
	root       string
}

type sessionLog = SessionLog

// Path returns the path to the session file on disk.
func (l *SessionLog) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Speaker identifies who emitted a message in the conversation.
type Speaker int

const (
	FromClient Speaker = iota
	FromReader
	FromModel
	FromThinking
	FromTool
	FromResult
	FromDiff
	FromFailure
)

type speaker = Speaker

const (
	fromClient   = FromClient
	fromReader   = FromReader
	fromModel    = FromModel
	fromThinking = FromThinking
	fromTool     = FromTool
	fromResult   = FromResult
	fromDiff     = FromDiff
	fromFailure  = FromFailure
)

// OpenSession creates a session log for recording.
func OpenSession(backend, model, root string) *SessionLog {
	return newSessionLog(backend, model, root)
}

// sessionRotationSize is the threshold at which a session file is rotated.
// 256 KB balances frequency against gzip overhead.
var sessionRotationSize int64 = 256 * 1024

// sessionHeader is the first line of the file, and the only place the run's
// own identity is written. Readers keyed on "v" can tell a v1 file from
// whatever replaces it without parsing the body, which is what a version
// field is for; the rest is what you need to make sense of the lines below
// it and is not repeated on any of them.
type sessionHeader struct {
	Version int    `json:"v"`
	Started string `json:"started"`
	Backend string `json:"backend"`
	Model   string `json:"model"`
	Root    string `json:"root"`
	PID     int    `json:"pid,omitempty"`
}

type sessionEntry struct {
	At     string `json:"t"`
	Who    string `json:"who"`
	Text   string `json:"text,omitempty"`
	Name   string `json:"name,omitempty"`
	Ms     int64  `json:"ms,omitempty"`
	Status string `json:"status,omitempty"`
}

// newSessionLog opens this run's transcript, or returns nil if it cannot.
//
// The directory is 0700 and the file 0600 because of what is in it: the
// questions a person typed at a work machine and what a model answered. Those
// are not world-readable on a shared host, and a mode set after the fact
// leaves a window where they were.
//
// The name is a timestamp plus the pid. The pid is what keeps two clients
// started in the same second from appending into one file: the stamp has
// second resolution, and two shells launched from one command is a normal
// thing to do rather than a rare race.
//
// The stamp is UTC and carries no punctuation, which is two deliberate
// departures from RFC3339. Local time sorts wrong: the offset changes at every
// DST boundary and on every flight, so a directory named in local time is not
// in chronological order however it looks, and sorting the names is the only
// index this has. And RFC3339 spells the offset with a colon, which is a
// character Windows refuses outright, SMB shares refuse for the same reason,
// and the Finder renders as a slash.
//
// The header is written here rather than lazily on the first line, so an
// abandoned session still leaves a file saying what it was going to be. A
// session where nobody typed anything is a fact worth having; a zero-length
// file is not.
//
// The root is recorded as given when it names a target: that names a machine on
// the other end of an SSH connection, not a directory here, and resolving it
// would record the path bulle was launched from and group the session with that
// project instead of with its VM.
func newSessionLog(backend, model, root string) *sessionLog {
	dir, err := settings.HomeDir()
	if err != nil {
		return nil
	}
	dir = filepath.Join(dir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	absRoot := root
	if !settings.IsTargetRoot(root) {
		if abs, err := filepath.Abs(root); err == nil {
			absRoot = abs
		}
	}
	now := time.Now()
	name := generateSessionID(dir) + ".jsonl"
	log := &sessionLog{
		path:    filepath.Join(dir, name),
		backend: backend,
		model:   model,
		root:    absRoot,
	}
	if !log.write(sessionHeader{
		Version: 1,
		Started: now.Format(time.RFC3339Nano),
		Backend: backend,
		Model:   model,
		Root:    absRoot,
		PID:     os.Getpid(),
	}) {
		return nil
	}
	return log
}

// HasWriteError reports whether the last write failed.
func (l *sessionLog) HasWriteError() bool {
	if l == nil {
		return false
	}
	return l.writeError
}

// Line records a question or an answer, and nothing else.
//
// The other six speakers are dropped on the floor, and that is the feature
// rather than an omission. fromThinking is the model's reasoning, fromResult
// is a tool's output and fromDiff is the content of a file that was edited —
// so a single `run_command` printing an environment variable, a `read_file`
// over a private key, or a diff touching a .env would put a secret in a file
// that outlives the session and that nobody remembers is there. Filtering it
// out afterwards means writing it first, which is exactly the moment the
// damage is done; the only reliable redaction is never collecting it.
//
// fromClient and fromFailure are dropped for a duller reason: they are the
// client talking about itself, and a transcript of what a person asked is not
// improved by the banner and the notices around it.
// Line records a message line in the session log.
func (l *SessionLog) Line(who Speaker, text string) {
	if l == nil {
		return
	}
	var label string
	switch who {
	case FromReader:
		label = "question"
	case FromModel:
		label = "answer"
	default:
		return
	}
	l.write(sessionEntry{At: stamped(), Who: label, Text: text})
}

// Tool records a tool execution in the session log.
func (l *SessionLog) Tool(name string, took time.Duration) {
	if l == nil {
		return
	}
	l.write(sessionEntry{At: stamped(), Who: "tool", Name: name, Ms: took.Milliseconds()})
}

// write appends one entry, reporting whether it landed.
//
// Every failure is swallowed by the callers. A transcript is bookkeeping
// beside the session, and a full disk or a revoked permission must not take
// down the thing the person is actually doing — the next line writes again,
// and a file with a gap in it is worth more than a client that fell over
// while producing one.
//
// It reports a bool rather than an error because there is exactly one caller
// that can act on the answer: newSessionLog, deciding whether this machine
// gets a log at all. Returning an error the other two would have to discard
// invites `_ =` at both, which is the spelling that hides a real mistake
// among two deliberate ones.
func (l *sessionLog) write(entry any) bool {
	line, err := json.Marshal(entry)
	if err != nil {
		l.writeError = true
		return false
	}
	if err := appendLine(l.path, line); err != nil {
		l.writeError = true
		return false
	}
	l.lastSize += int64(len(line)) + 1
	if l.lastSize >= sessionRotationSize {
		l.rotate()
	}
	return true
}
