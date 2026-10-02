package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// opened points HOME at a temporary tree and returns this run's log together
// with a reader of the lines it has written. HOME is redirected for every
// test in this file: a test that records a question into the real
// ~/.bulle/sessions/ has polluted the transcript directory of whoever ran
// it, and the one property this whole file exists to defend is what does and
// does not end up in that directory.
func opened(t *testing.T) (*sessionLog, func() []string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("a writable home must produce a log")
	}
	return log, func() []string {
		t.Helper()
		data, err := os.ReadFile(log.path)
		if err != nil {
			t.Fatalf("reading the transcript: %v", err)
		}
		return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
}

// The header is the only place the run says what it was, and it is written at
// open rather than at the first line — so a session where nobody typed
// anything still leaves a file that identifies itself. A reader keying on "v"
// to tell this schema from its successor needs it on line one, always.
func TestTheFirstLineNamesTheSchemaBackendModelAndRoot(t *testing.T) {
	log, lines := opened(t)

	var header sessionHeader
	if err := json.Unmarshal([]byte(lines()[0]), &header); err != nil {
		t.Fatalf("the first line is not JSON: %v", err)
	}
	if header.Version != 1 {
		t.Errorf("version = %d, want 1", header.Version)
	}
	if header.Backend != "anthropic" || header.Model != "claude-opus-5" || header.Root != "/repo" {
		t.Errorf("header does not identify the run: %+v", header)
	}
	if _, err := time.Parse(time.RFC3339Nano, header.Started); err != nil {
		t.Errorf("started = %q, want RFC3339Nano: %v", header.Started, err)
	}
	if !strings.HasSuffix(log.path, ".jsonl") {
		t.Errorf("path = %q, want a .jsonl file", log.path)
	}
}

// The whole point of the file: what a person asked, and what came back. Both
// halves, labelled apart, or a transcript reads as a monologue.
func TestAQuestionAndAnAnswerAreBothRecorded(t *testing.T) {
	log, lines := opened(t)

	log.Line(fromReader, "why is the build slow")
	log.Line(fromModel, "because cgo is on")

	written := lines()
	if len(written) != 3 {
		t.Fatalf("got %d lines, want a header and two entries: %q", len(written), written)
	}
	for i, want := range []sessionEntry{
		{Who: "question", Text: "why is the build slow"},
		{Who: "answer", Text: "because cgo is on"},
	} {
		var got sessionEntry
		if err := json.Unmarshal([]byte(written[i+1]), &got); err != nil {
			t.Fatalf("entry %d is not JSON: %v", i, err)
		}
		if got.Who != want.Who || got.Text != want.Text {
			t.Errorf("entry %d = %+v, want who=%q text=%q", i, got, want.Who, want.Text)
		}
	}
}

// This is the privacy decision, and it is the one test that must never be
// relaxed. Reasoning, tool output and file diffs carry whatever happened to
// be in the buffer — an environment variable a command echoed, the contents
// of a .env a diff touched — and a file outlives the session that made it.
// Filtering afterwards means writing it first, so it is never collected.
func TestReasoningToolOutputAndDiffsNeverReachTheDisk(t *testing.T) {
	log, lines := opened(t)

	secret := "AWS_SECRET_ACCESS_KEY=hunter2"
	for _, who := range []speaker{fromClient, fromThinking, fromTool, fromResult, fromDiff, fromFailure} {
		log.Line(who, secret)
	}

	written := lines()
	if len(written) != 1 {
		t.Fatalf("only the header may be on disk, got %d lines: %q", len(written), written)
	}
	if strings.Contains(written[0], "hunter2") {
		t.Fatal("a secret reached the transcript")
	}
}

// A finished tool is recorded as a shape, not as content: the name and how
// long it took. Its input is the single most reliable place in a session to
// find a path, a token or a command line, so the entry has no room for one.
func TestAFinishedToolRecordsOnlyItsNameAndDuration(t *testing.T) {
	log, lines := opened(t)

	log.Tool("run_command", 1500*time.Millisecond)

	var got sessionEntry
	if err := json.Unmarshal([]byte(lines()[1]), &got); err != nil {
		t.Fatalf("the entry is not JSON: %v", err)
	}
	if got.Who != "tool" || got.Name != "run_command" || got.Ms != 1500 {
		t.Errorf("entry = %+v, want tool/run_command/1500ms", got)
	}
	if got.Text != "" {
		t.Errorf("text = %q, want nothing: a tool entry carries no content", got.Text)
	}
}

// The modes are set at creation, not fixed up afterwards, because a chmod
// after the fact leaves a window where the questions somebody typed at a
// shared machine were world-readable.
func TestATranscriptIsReadableOnlyByItsOwner(t *testing.T) {
	log, _ := opened(t)

	file, err := os.Stat(log.path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := file.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %04o, want 0600", got)
	}
	dir, err := os.Stat(filepath.Dir(log.path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dir.Mode().Perm(); got != 0o700 {
		t.Errorf("dir mode = %04o, want 0700", got)
	}
}

// say() calls line() on every transcript line without a nil check, so a
// machine where the file could not be opened — read-only home, full disk —
// would take the client down on the first thing anybody said. Nil is a
// supported state, not an error path.
func TestANilLogRecordsNothingWithoutPanicking(t *testing.T) {
	var log *sessionLog

	log.Line(fromReader, "still here?")
	log.Line(fromModel, "still here.")
	log.Tool("read_file", time.Second)
}

func checkRotatedFiles(t *testing.T, dir string) {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading sessions dir: %v", err)
	}
	hasGz, hasCurrent := false, false
	var gzMode os.FileMode
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".gz") {
			hasGz = true
			if info, err := f.Info(); err == nil {
				gzMode = info.Mode().Perm()
			}
		}
		if strings.HasSuffix(f.Name(), ".jsonl") {
			hasCurrent = true
		}
	}
	if !hasGz {
		t.Error("expected a gzipped rotated session file")
	}
	if gzMode != 0o600 {
		t.Errorf("rotated gz file mode = %04o, want 0600", gzMode)
	}
	if !hasCurrent {
		t.Error("expected a current session file after rotation")
	}
}

// TestSessionRotation verifies that session logs rotate at 256KB and gzip the old file.
func TestSessionRotation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("a writable home must produce a log")
	}

	sessionRotationSize = 1024
	t.Cleanup(func() { sessionRotationSize = 256 * 1024 })

	for range 50 {
		log.Line(fromReader, strings.Repeat("x", 200))
	}

	checkRotatedFiles(t, filepath.Join(home, ".bulle", "sessions"))
}

// TestHasWriteError verifies that write errors are tracked.
func TestHasWriteError(t *testing.T) {
	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("a writable home must produce a log")
	}

	if log.HasWriteError() {
		t.Error("HasWriteError() should be false initially")
	}

	log.Line(fromReader, "test")
	if log.HasWriteError() {
		t.Error("HasWriteError() should be false after successful write")
	}

	var nilLog *sessionLog
	if nilLog.HasWriteError() {
		t.Error("nil log HasWriteError() should be false")
	}
}
