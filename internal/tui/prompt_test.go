package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/settings"
)

// TestPromptBackdropCoversEveryRow: a two-line question — the first line away
// from the cursor, the second wrapping over the edge — must carry the backdrop
// on every row. The textarea only backdrops the cursor's own logical line, so
// without the base backdrop the rows off the cursor fall back to the terminal
// default and the prompt reads as a patchwork.
func TestPromptBackdropCoversEveryRow(t *testing.T) {
	m := sized()
	m.prompt.SetValue("first line\nsecond line that is quite long and wraps over onto a further row, going past the edge")
	m.resize(tea.WindowSizeMsg{Width: 60, Height: 24})

	rows := strings.Split(m.prompt.View(), "\n")
	if got := len(rows); got != 3 {
		t.Fatalf("prompt drew %d rows, want the two lines across three", got)
	}
	for i, row := range rows {
		if !strings.Contains(row, "\x1b[40m") {
			t.Errorf("row %d = %q, want the backdrop on every row", i, row)
		}
		if got := visible(row); !strings.Contains(got, "▌") {
			t.Errorf("row %d = %q, want the muted ▌ gutter on every row", i, row)
		}
	}
}

// TestResolveEditorWithVariousEnvironmentVariables tests resolveEditor's
// precedence: empty sources → vi, EDITOR env wins, VISUAL env wins when
// EDITOR is empty, and config.Editor wins over both env vars.
func TestResolveEditorWithVariousEnvironmentVariables(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	if r := resolveEditor(settings.Config{}); r != "vi" {
		t.Errorf("resolveEditor = %q, want vi", r)
	}
	t.Setenv("EDITOR", "/usr/local/bin/vim")
	if r := resolveEditor(settings.Config{}); r != "/usr/local/bin/vim" {
		t.Errorf("resolveEditor = %q, want EDITOR to win", r)
	}
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "/usr/local/bin/nano")
	if r := resolveEditor(settings.Config{}); r != "/usr/local/bin/nano" {
		t.Errorf("resolveEditor = %q, want VISUAL to win", r)
	}
	cfg := settings.Config{Editor: settings.Editor{Editor: "/opt/homebrew/bin/nvim"}}
	t.Setenv("EDITOR", "/usr/local/bin/vim")
	if r := resolveEditor(cfg); r != "/opt/homebrew/bin/nvim" {
		t.Errorf("resolveEditor = %q, want config to win", r)
	}
	t.Setenv("EDITOR", "")
	if r := resolveEditor(cfg); r != "/opt/homebrew/bin/nvim" {
		t.Errorf("resolveEditor = %q, want config without env to win", r)
	}
}

// TestEditInExternalEditorWithMockEditor: a mock editor that prints
// its argument file's contents to stdout (cat) is enough to prove the
// round-trip — the content is written out, the editor runs, and the
// modified contents come back.
func TestEditInExternalEditorWithMockEditor(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available on this system")
	}

	original := "hello, world"
	edited, err := editInExternalEditor(original, "cat")
	if err != nil {
		t.Fatalf("editInExternalEditor: %v", err)
	}
	if edited != original {
		t.Errorf("edited = %q, want %q", edited, original)
	}
}

// TestEditInExternalEditorWithMockEditorThatChanges: a mock editor
// that appends a marker line proves the editor's output is read back.
func TestEditInExternalEditorWithMockEditorThatChanges(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "bulle-editor-marker.sh")
	script := "#!/bin/sh\ncat \"$1\"\necho 'edited by mock' >> \"$1\""
	if err := os.WriteFile(marker, []byte(script), 0o700); err != nil {
		t.Fatalf("writing mock editor: %v", err)
	}

	original := "before\n"
	edited, err := editInExternalEditor(original, marker)
	if err != nil {
		t.Fatalf("editInExternalEditor: %v", err)
	}
	if edited == original {
		t.Error("edited content unchanged, want the mock editor's modification")
	}
	if !strings.Contains(edited, "edited by mock") {
		t.Errorf("edited = %q, want the mock editor's marker", edited)
	}
}

// TestEditInExternalEditorWithEmptyPrompt: an empty prompt string is
// a valid input — the editor opens on an empty file and whatever the
// editor writes is returned. The function does not reject emptiness.
func TestEditInExternalEditorWithEmptyPrompt(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available on this system")
	}

	edited, err := editInExternalEditor("", "cat")
	if err != nil {
		t.Fatalf("editInExternalEditor: %v", err)
	}
	if edited != "" {
		t.Errorf("edited = %q, want empty for an empty prompt", edited)
	}
}

// TestEditInExternalEditorMissingEditor: when no editor is configured,
// the function returns an error rather than falling through to a
// phantom command.
func TestEditInExternalEditorMissingEditor(t *testing.T) {
	_, err := editInExternalEditor("content", "")
	if err == nil {
		t.Fatal("editInExternalEditor with empty editor: expected an error")
	}
	if !strings.Contains(err.Error(), "no editor configured") {
		t.Errorf("error = %v, want 'no editor configured'", err)
	}
}

// TestEditInExternalEditorTempFileWriteFailure: when the temp directory
// is unreachable, the function reports the write failure instead of
// launching the editor.
func TestEditInExternalEditorTempFileWriteFailure(t *testing.T) {
	saved := os.Getenv("TMPDIR")
	t.Setenv("TMPDIR", "/dev/null/impossible/path")
	defer func() {
		if saved == "" {
			os.Unsetenv("TMPDIR")
		} else {
			os.Setenv("TMPDIR", saved)
		}
	}()

	_, err := editInExternalEditor("content", "cat")
	if err == nil {
		t.Fatal("editInExternalEditor with unwritable TMPDIR: expected an error")
	}
	if !strings.Contains(err.Error(), "creating temp file") {
		t.Errorf("error = %v, want a temp-file creation error", err)
	}
}

// TestEditInExternalEditorMissingEditorBinary: when the editor binary
// does not exist, the function reports the launch failure rather
// than crashing.
func TestEditInExternalEditorMissingEditorBinary(t *testing.T) {
	_, err := editInExternalEditor("content", "/no/such/editor/exists/here")
	if err == nil {
		t.Fatal("editInExternalEditor with missing binary: expected an error")
	}
}

// TestNewModelSetsEditorPath verifies that NewModel populates editorPath on the inflight run.
func TestNewModelSetsEditorPath(t *testing.T) {
	cfg := SessionConfig{
		Editor: EditorConfig{
			Editor:        "/usr/local/bin/custom-editor",
			PromptEditKey: "custom_key",
		},
	}
	m := NewModel(nil, "banner", nil, cfg)
	if m.run.editorPath != "/usr/local/bin/custom-editor" {
		t.Errorf("m.run.editorPath = %q, want /usr/local/bin/custom-editor", m.run.editorPath)
	}
	if m.run.promptEditKey != "custom_key" {
		t.Errorf("m.run.promptEditKey = %q, want custom_key", m.run.promptEditKey)
	}
}
