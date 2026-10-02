package tui

import (
	"strings"
	"testing"
)

// The editor's commands, each one taking the path the terminal already had: a
// stop is the escape key's own cancel, a prompt is the submit a typed question
// goes through, and an open moves the window the reader is looking at. Nothing
// here is a parallel mechanism, which is the whole point of the socket being an
// attachment to the session rather than a way around it.

// Stop takes the escape path rather than a cancel of its own: the run is
// cancelled, the stop is named the way esc names it, and esc's own mark is
// taken, which is what the force-quit window reads.
func TestAnEditorStopTakesTheEscapePath(t *testing.T) {
	cancelled := false
	m := attached(&fakeSurface{})
	m.run.busy = true
	m.run.cancel = func() { cancelled = true }

	m.Stop()

	if !cancelled {
		t.Error("the editor's stop did not cancel the run")
	}
	if m.run.stop != abandoned {
		t.Errorf("stop = %q, want the escape path's own stop", m.run.stop)
	}
	if m.run.interrupted.IsZero() {
		t.Error("esc's own mark was not taken, so this is a second cancel rather than the same one")
	}
}

// A prompt from an editor goes through the same submit a typed one does: it is
// queued behind the run in flight, and the text carries where the cursor was,
// so what the model reads and what the reader sees are the same line.
func TestAnEditorPromptTakesTheSamePathAsATypedOne(t *testing.T) {
	m := attached(&fakeSurface{})
	m.run.busy = true

	m.Send("tighten this", "internal/tui/ide.go", 42, "main")

	queued := m.Items()
	if len(queued) != 1 {
		t.Fatalf("queue = %v, want the editor's prompt queued behind the run", queued)
	}
	for _, want := range []string{"internal/tui/ide.go:42", "main", "tighten this"} {
		if !strings.Contains(queued[0], want) {
			t.Errorf("queued prompt = %q, want it to name %q", queued[0], want)
		}
	}
}

// Open moves the transcript window to the newest row that mentions the path,
// and a path the transcript never mentions moves nothing: an editor following a
// change into a session that never touched that file must not throw the reader
// somewhere arbitrary.
func TestAnEditorOpenMovesTheTranscriptToThePath(t *testing.T) {
	m := attached(&fakeSurface{})
	m.mode = modeTUI
	m.hold = []string{"bulle 0.76.0", "✎ edit_file internal/tui/ide.go", "", "answer", "done"}

	m.Open("internal/tui/ide.go", 12)

	opened := m.scrollTop
	if rows := window(m.hold, 3, opened); !strings.Contains(strings.Join(rows, "\n"), "internal/tui/ide.go") {
		t.Errorf("window = %v, want the row that mentions the path brought on screen", rows)
	}

	m.Open("internal/tui/nothing.go", 1)

	if m.scrollTop != opened {
		t.Error("a path the transcript never mentions moved the window")
	}
}

// The session hands the model to the editor as its command handler, which is
// what makes an editor able to drive it at all: there is no other path from a
// socket command to this loop.
func TestBootHandsTheModelToTheEditor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	surface := &fakeSurface{}
	config := testSession()
	m := NewModel(nil, "banner", nil, config)

	boot(m, UISession{IDE: surface, SessionConfig: config})

	if surface.commands == nil {
		t.Fatal("the attached editor was never handed a command handler")
	}
	if _, ok := surface.commands.(*Model); !ok {
		t.Errorf("handler is %T, want the model", surface.commands)
	}
	if m.ide.surface != IDESurface(surface) {
		t.Error("the model does not hold the surface it was booted with")
	}
}
