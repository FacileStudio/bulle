package sessions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

func TestListSessionFilesEmpty(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())
	files := ListSessionFiles("")
	if len(files) != 0 {
		t.Errorf("got %v, want empty", files)
	}
}

func TestLoadSessionMissingFile(t *testing.T) {
	msgs := LoadSession("/nonexistent/file.jsonl")
	if msgs != nil {
		t.Errorf("got %v, want nil", msgs)
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	log.Line(fromReader, "what is 2+2?")
	log.Line(fromModel, "4")

	files := ListSessionFiles("")
	if len(files) == 0 {
		t.Fatal("expected at least one session file")
	}

	msgs := LoadSession(files[0])
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}

	formatted := FormatSessionEntry(files[0])
	if !strings.Contains(formatted, "anthropic") || !strings.Contains(formatted, "claude-opus-5") {
		t.Errorf("formatted = %q, want backend and model", formatted)
	}
}

func TestFormatSessionEntryNonexistent(t *testing.T) {
	formatted := FormatSessionEntry("/nonexistent/test.jsonl")
	if !strings.Contains(formatted, "error reading file") {
		t.Errorf("formatted = %q, want error indicator", formatted)
	}
}

func TestFormatSessionEntryShortStarted(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/short.jsonl"
	content := `{"v":1,"started":"short","backend":"google","model":"gemini","root":"."}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	formatted := FormatSessionEntry(path)
	if !strings.Contains(formatted, "google") || !strings.Contains(formatted, "short") {
		t.Errorf("formatted = %q, want backend and short started", formatted)
	}
}

func TestListSessionFilesFilter(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}

	files := ListSessionFiles("/nonexistent/project/dir")
	if len(files) != 0 {
		t.Fatalf("expected 0 files for unmatched project dir, got %d", len(files))
	}
	all := ListSessionFiles("")
	if len(all) != 1 {
		t.Fatalf("expected 1 file for all sessions, got %d", len(all))
	}
}

func TestResolveSessionByIdAndPath(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	path := log.Path()

	if resolved := ResolveSession(path); resolved != path {
		t.Errorf("resolved %q by path, want %q", resolved, path)
	}
	if resolved := ResolveSession(filepath.Base(path)); resolved != path {
		t.Errorf("resolved %q by id, want %q", resolved, path)
	}
	if resolved := ResolveSession(filepath.Base(path) + ".jsonl"); resolved != path {
		t.Errorf("resolved %q by id with suffix, want %q", resolved, path)
	}
	if resolved := ResolveSession("no-such-session"); resolved != "" {
		t.Errorf("resolved %q for a missing id, want empty", resolved)
	}
	if resolved := ResolveSession(""); resolved != "" {
		t.Errorf("resolved %q for empty, want empty", resolved)
	}
}

func TestRestoreAtLaunch(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)

	res := RestoreAtLaunch("", "/repo", false)
	if res.Conversation != nil || res.Name != "" || res.Path != "" || res.Error != "" {
		t.Errorf("no-op restore = %+v, want empty struct", res)
	}

	res = RestoreAtLaunch("no-such-session", "/repo", false)
	if res.Conversation != nil || res.Name != "" || res.Path != "" || res.Error == "" {
		t.Errorf("missing resume = %+v, want error", res)
	}

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	log.Line(fromReader, "hi")

	res = RestoreAtLaunch(filepath.Base(log.Path()), "/repo", false)
	if res.Conversation == nil || len(res.Conversation) != 1 || res.Name == "" || res.Path != log.Path() || res.Error != "" {
		t.Errorf("explicit resume = %+v, want one message and no error", res)
	}

	res = RestoreAtLaunch(log.Path(), "/repo", false)
	if res.Conversation == nil || len(res.Conversation) != 1 || res.Name == "" || res.Path != log.Path() || res.Error != "" {
		t.Errorf("absolute-path resume = %+v, want one message and no error", res)
	}

	res = RestoreAtLaunch("", "/repo", true)
	if res.Conversation == nil || len(res.Conversation) != 1 || res.Name == "" || res.Path != log.Path() || res.Error != "" {
		t.Errorf("auto resume = %+v, want the newest session and no error", res)
	}
}

func TestOpenResumeSession(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)

	if log := OpenResumeSession("", "anthropic", "claude", "/repo"); log != nil {
		t.Errorf("expected nil log for empty path, got %v", log)
	}

	orig := newSessionLog("anthropic", "claude-opus-5", "/repo")
	orig.Line(fromReader, "first question")
	orig.Line(fromModel, "first answer")

	resumed := OpenResumeSession(orig.Path(), "anthropic", "claude-opus-5", "/repo")
	if resumed == nil {
		t.Fatal("expected non-nil resumed log")
	}
	resumed.Line(fromReader, "second question")
	resumed.Line(fromModel, "second answer")

	msgs := LoadSession(orig.Path())
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages in resumed session log, got %d", len(msgs))
	}
}
