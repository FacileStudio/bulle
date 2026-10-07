package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/layout"
)

// The kinds of message an attached editor sends this session. The first three
// are the protocol's own command types, spelled the way it spells them; answer
// is the one that never came off the socket, being what the editor's Approve
// returned handed back to the loop.
const (
	ideSend   = "send"
	ideOpen   = "open"
	ideStop   = "stop"
	ideAnswer = "answer"
)

// Commands is how an attached editor drives this session: a prompt it sent, a
// place it asked to be shown, a run it asked to stop. It is declared here
// method for method with the ide package's own Commands, because the terminal
// owns the prompt loop and that package must not be imported back into it.
type Commands interface {
	Send(text, path string, line int, branch string)
	Open(path string, line int)
	Stop()
}

// IDEDecision is an attached editor's answer to a pending tool call, in the
// terminal's own words. Unanswered is not a refusal: it means no editor was
// attached to ask, so the call is still parked and the terminal prompt already
// on screen is what decides it.
type IDEDecision int

const (
	IDEUnanswered IDEDecision = iota
	IDERefused
	IDEAllowed
)

// IDESurface is the editor a session publishes to, as the terminal reads it:
// the one call that asks it to answer a pending tool call, and the one that
// hands it the session's command handler. A nil surface is a session nobody
// watches, and costs a nil check and nothing else.
type IDESurface interface {
	Approve(ctx context.Context, id, tool, input string) IDEDecision
	SetCommands(Commands)
	Turn(n int)
	Done(reason string, cost float64)
}

// ideWiring is the editor attachment as the model holds it: the surface
// itself, the running program's own message send, and the little state the two
// halves of an approval need to stay paired — the id the editor was asked
// about, and the counter those ids are numbered from.
//
// It carries the run's shape too, because the editor is told about it: how many
// model turns have started, whether one is open, and whether a failure ended
// the run. They are reset together where a run starts.
type ideWiring struct {
	surface IDESurface
	deliver func(tea.Msg)
	asked   string
	seq     int
	turns   int
	open    bool
	failed  bool
}

// ideEvent is one thing the attached editor did. The four kinds travel as one
// type so the session's dispatcher stays an arm wide: a command arrives on the
// socket's goroutine and an answer comes back from the command that asked, and
// both reach the loop as this.
type ideEvent struct {
	kind     string
	text     string
	path     string
	branch   string
	id       string
	line     int
	decision IDEDecision
}

// Send submits a prompt an attached editor sent, carrying the place its cursor
// was. The event is handed to the update loop rather than acted on here: it
// arrives on the socket's own goroutine, and the loop is the only thing that
// may touch the transcript.
func (m *Model) Send(text, path string, line int, branch string) {
	m.editor(ideEvent{kind: ideSend, text: text, path: path, branch: branch, line: line})
}

// Open asks the session to show the place the editor is looking at.
func (m *Model) Open(path string, line int) {
	m.editor(ideEvent{kind: ideOpen, path: path, line: line})
}

// Stop asks the session to stop the run in flight.
func (m *Model) Stop() {
	m.editor(ideEvent{kind: ideStop})
}

// editor hands one editor event to the update loop. A model with no loop
// running has nowhere to hand it to and drops it: acting on it here would touch
// the transcript from the socket's goroutine, which is the one thing this hop
// exists to prevent.
func (m *Model) editor(ev ideEvent) {
	if m.ide.deliver == nil {
		return
	}
	m.ide.deliver(ev)
}

// editorEvent routes one thing the attached editor did to the same place a
// keypress or a typed question reaches. Every arm is a forward — the decisions
// stay where they already were — so an editor drives this session through the
// paths the terminal uses rather than beside them.
func (m *Model) editorEvent(ev ideEvent) tea.Cmd {
	switch ev.kind {
	case ideSend:
		return m.submit(editorPrompt(ev.text, ev.path, ev.line, ev.branch))
	case ideOpen:
		m.editorOpen(ev.path)
	case ideStop:
		m.escaped()
	case ideAnswer:
		m.editorAnswer(ev)
	}
	return nil
}

// editorOpen moves the transcript window to the newest place it mentions the
// path the editor is looking at. It scrolls to a row this session has already
// painted, so a path nowhere in the transcript moves nothing: an editor
// following a change into a session that never touched that file must not
// throw the reader somewhere arbitrary. Inline mode has no window of its own to
// move, the terminal owning that scrollback, and the line is not read — the
// transcript has no finer address than the row a block starts on.
func (m *Model) editorOpen(path string) {
	if path == "" || m.mode != modeTUI {
		return
	}
	for i, row := range slices.Backward(m.hold) {
		if strings.Contains(layout.Unstyled(row), path) {
			m.scrollTop = len(m.hold) - 1 - i
			return
		}
	}
}

// editorPrompt is the one line a prompt from an editor carries: where its
// cursor was, then what was asked. It is deliberately one string, handed to the
// same submit a typed question goes through — the model reads the file, the
// line and the branch, the reader sees exactly the text that was sent, and
// nothing an editor attaches can reach one of them without the other.
func editorPrompt(text, path string, line int, branch string) string {
	where := path
	if where != "" && line > 0 {
		where = fmt.Sprintf("%s:%d", where, line)
	}
	if branch != "" {
		if where == "" {
			where = "branch " + branch
		} else {
			where += " on " + branch
		}
	}
	if where == "" {
		return text
	}
	return where + "\n\n" + text
}
