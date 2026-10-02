package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
	"github.com/charmbracelet/x/ansi"
)

// visible is a rendered screen with the styling stripped, which is what a
// test checking for words rather than colours wants: markdown rendering wraps
// a paragraph in a colour escape per line, and a literal check for a phrase
// spanning two of those lines never finds it in the raw ANSI.
func visible(screen string) string { return ansi.Strip(screen) }

// sized is a model with a window, because everything that renders needs one.
func sized() *Model {
	m := NewModel(nil, "test · model", nil, testSession())
	m.resize(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

// bareBanner is the same model without a window, for tests that only drive
// logic rather than rendering.
func bareBanner() *Model {
	return NewModel(nil, "banner", nil, testSession())
}

// A start message is the first thing said: the launch block prints above the
// banner, so an ascii-art welcome leads and the version banner follows. An
// empty start message prints nothing — the banner stays the first and only
// thing.
func TestStartMessagePrintsAboveTheBanner(t *testing.T) {
	config := testSession()
	config.StartMessage = "welcome\nto bulle"
	m := NewModel(nil, "banner", nil, config)
	said := visible(strings.Join(m.unprinted, "\n"))
	message, bannerAt := strings.Index(said, "to bulle"), strings.Index(said, "banner")
	if message < 0 || bannerAt < 0 || message > bannerAt {
		t.Errorf("launch = %q, want the start message above the banner", said)
	}
	if got := visible(m.unprinted[0]); !strings.Contains(got, "to bulle") {
		t.Errorf("first thing said = %q, want the start message", got)
	}
}

// Text arrives a few characters at a time. A transcript with one entry per
// delta is unreadable, so they have to accumulate into a single answer.
func TestStreamedTextAccumulatesIntoOneAnswer(t *testing.T) {
	m := sized()
	for _, delta := range []string{"the ", "whole ", "answer"} {
		m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: delta})
	}

	if got := m.run.answer.String(); got != "the whole answer" {
		t.Errorf("answer = %q, want the deltas joined", got)
	}
	if lines := spoken(m); len(lines) != 0 {
		t.Errorf("transcript = %v, want the deltas kept out of it", lines)
	}
}

// A run that ends has to leave its answer in the conversation, or the next
// question is asked of a model that never said anything.
func TestAFinishedAnswerJoinsTheConversation(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "an answer"})
	m.settle()

	if len(m.conversation) != 1 {
		t.Fatalf("conversation = %v, want the answer appended", m.conversation)
	}
	if m.conversation[0].Role != nacelle.RoleAssistant || said(m.conversation[0]) != "an answer" {
		t.Errorf("message = %+v, want the assistant's answer", m.conversation[0])
	}
	if m.run.busy {
		t.Error("still busy after the run settled")
	}
	if m.run.answer.Len() != 0 {
		t.Error("the answer was left in the buffer for the next turn to repeat")
	}
}

// Watching a tool run is most of why a terminal is the first consumer — but a
// tool that worked has one thing to report, and reporting it twice doubled the
// height of every transcript to say nothing had gone wrong.
func TestASuccessfulToolIsOneLineCarryingItsDuration(t *testing.T) {
	m := sized()
	m.absorb(nacelle.Event{
		Kind: nacelle.KindToolCall,
		Tool: &nacelle.ToolEvent{Name: "read_file", Input: `{"path":"go.mod"}`},
	})
	m.absorb(nacelle.Event{
		Kind: nacelle.KindToolResult,
		Tool: &nacelle.ToolEvent{Name: "read_file", Duration: 3 * time.Millisecond},
	})

	lines := spoken(m)
	if len(lines) != 1 {
		t.Fatalf("transcript = %v, want the call and its duration on one line", lines)
	}
	if !strings.Contains(lines[0], "read_file") {
		t.Errorf("call line = %q, want the tool named", lines[0])
	}
	if !strings.Contains(lines[0], "3ms") {
		t.Errorf("call line = %q, want how long it took", lines[0])
	}
}

// KindDone carries the run's total, not another turn to add on. Adding it
// would bill every run for its last turn twice.
func TestTheRunTotalReplacesTheRunningCount(t *testing.T) {
	m := sized()
	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{InputTokens: 10}})
	m.absorb(nacelle.Event{Kind: nacelle.KindTurn, Usage: nacelle.Usage{InputTokens: 10}})
	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Usage: nacelle.Usage{InputTokens: 20}})

	if got := m.run.usage.Total(); got != 20 {
		t.Errorf("total = %d, want the run total rather than the sum of both", got)
	}
}

// The terminal is in raw mode, so nothing quits on Ctrl+C unless the model
// says so — and while an answer is streaming it should stop that answer rather
// than throwing the session away.
func TestCtrlCStopsTheRunBeforeItQuits(t *testing.T) {
	press := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	if press.String() != "ctrl+c" {
		t.Fatalf("constructed key = %q, want ctrl+c", press.String())
	}

	m := sized()
	stopped := false
	m.run.cancel, m.run.busy = func() { stopped = true }, true

	handled, cmd := m.key(press)
	if !handled {
		t.Fatal("ctrl+c was passed through to the prompt")
	}
	if !stopped {
		t.Error("the run was not cancelled")
	}
	if cmd != nil {
		t.Error("the program quit instead of stopping the run")
	}

	m.run.busy = false
	if _, cmd = m.key(press); cmd == nil {
		t.Error("ctrl+c with nothing running did not quit")
	}
}

// The answer streams into a buffer that render only draws while it is filling.
// Clearing that buffer without moving the text somewhere permanent erases the
// answer from the screen at the moment it finished, which is exactly what a
// first real session found.
func TestAFinishedAnswerStaysOnScreen(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.absorb(nacelle.Event{Kind: nacelle.KindText, Text: "the whole answer"})
	m.settle()

	if !strings.Contains(onScreen(m), "the whole answer") {
		t.Errorf("screen = %q, want the finished answer still visible", onScreen(m))
	}
}

// The banner names what is about to be billed, before anything is typed.
func TestTheBackendAndModelAreShownBeforeTheFirstQuestion(t *testing.T) {
	m := sized()

	if !strings.Contains(onScreen(m), "test · model") {
		t.Errorf("screen = %q, want the backend and model named", onScreen(m))
	}
}

// A run cut off by the token ceiling arrives as a well-formed stream that
// simply ends, so the only thing separating a truncated answer from a finished
// one on screen is this line. "ready" under half a sentence is a lie.
func TestAnIncompleteRunSaysSoInTheStatusLine(t *testing.T) {
	incomplete := map[nacelle.Stop]string{
		nacelle.StopMaxTokens:  "token limit",
		nacelle.StopContext:    "out of context",
		nacelle.StopRefusal:    "refused",
		nacelle.StopIterations: "iteration limit",
		nacelle.StopOther:      "stopped early",
	}

	for stop, want := range incomplete {
		m := sized()
		m.absorb(nacelle.Event{Kind: nacelle.KindDone, Stop: stop})

		status := m.status()
		if !strings.Contains(status, want) {
			t.Errorf("status for %q = %q, want it to mention %q", stop, status, want)
		}
		if strings.Contains(status, "ready") {
			t.Errorf("status for %q = %q, want a truncated run not to read as a finished one", stop, status)
		}
	}
}

// The other half of the same bargain: a run that finished must not be dressed
// up as a problem, or the warning stops meaning anything.
func TestAFinishedRunLeavesTheStatusLineAlone(t *testing.T) {
	m := sized()
	m.absorb(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd})

	if status := m.status(); !strings.Contains(status, "ready") {
		t.Errorf("status = %q, want the usual ready line", status)
	}
}

// The context and system-prompt notes follow the banner, before anything the
// user does: the moment they answer is the first the run counter ticks, so
// the pre-turn cost of the prompt can only be read here.
func TestStartupNotesFollowTheBanner(t *testing.T) {
	m := NewModel(nil, "banner", nil, SessionConfig{
		Startup: LaunchContext{
			SystemTokens: 12000, ContextTokens: 4000,
			ContextPaths: []string{"/tmp/CLAUDE.md", "/tmp/AGENTS.md"},
		},
	})
	said := visible(strings.Join(m.unprinted, "\n"))
	banner, context, system := strings.Index(said, "banner"), strings.Index(said, "context: 2 files loaded"), strings.Index(said, "system prompt loaded · ~12.0k tokens")
	if banner < 0 || context < 0 || system < 0 || banner > context || context > system {
		t.Errorf("launch = %q, want banner, then context, then system prompt", said)
	}
	if !strings.Contains(said, "/tmp/CLAUDE.md, /tmp/AGENTS.md") {
		t.Errorf("launch = %q, want both context paths named", said)
	}
}

// A launch with no system prompt to report — the zero-config shape tests and
// stubs build — prints nothing about context or tokens, the same rule the
// banner's MCP line follows: silence for the ordinary case, not a claim of
// zero.
func TestStartupNotesStaySilentWithoutASystemPrompt(t *testing.T) {
	m := NewModel(nil, "banner", nil, SessionConfig{})
	said := visible(strings.Join(m.unprinted, "\n"))
	if strings.Contains(said, "context") || strings.Contains(said, "system prompt") {
		t.Errorf("launch = %q, want no context or system-prompt note", said)
	}
}
