package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/approval"
)

// fakeSurface is an attached editor under a test's control: it answers whatever
// the test set, and keeps what it was asked so the assertions can read it back.
// The commands it was handed are kept too, which is how a test sees that the
// session gave the editor something to drive.
type fakeSurface struct {
	answer   IDEDecision
	asked    int
	id       string
	tool     string
	input    string
	deadline time.Time
	commands Commands
	turns    []int
	reasons  []string
	cost     float64
}

func (f *fakeSurface) Approve(ctx context.Context, id, tool, input string) IDEDecision {
	f.asked++
	f.id, f.tool, f.input = id, tool, input
	f.deadline, _ = ctx.Deadline()
	return f.answer
}

func (f *fakeSurface) Turn(n int) {
	f.turns = append(f.turns, n)
}

func (f *fakeSurface) Done(reason string, cost float64) {
	f.reasons = append(f.reasons, reason)
	f.cost = cost
}

func (f *fakeSurface) SetCommands(commands Commands) {
	f.commands = commands
}

// attached is the model a test drives an editor against: one editor, and the
// program's own send replaced by a direct call into the loop, which is what the
// running program does with it.
func attached(surface *fakeSurface) *Model {
	m := sized()
	m.ide.surface = surface
	m.ide.deliver = func(msg tea.Msg) { m.Update(msg) }
	return m
}

func TestAnEditorApprovalDecidesTheCall(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply IDEDecision
		want  approval.Decision
	}{{"allow", IDEAllowed, approval.AllowedOnce}, {"deny", IDERefused, approval.Denied}} {
		t.Run(tc.name, func(t *testing.T) {
			surface := &fakeSurface{answer: tc.reply}
			m := attached(surface)
			decision := make(chan approval.Decision, 1)
			input := []byte(`{"command":"ls"}`)

			cmd := m.parkApproval(approval.Request{Name: "run_command", Input: input, Decision: decision})
			if cmd == nil {
				t.Fatal("the attached editor was never asked")
			}
			m.Update(cmd())

			if got := <-decision; got != tc.want {
				t.Errorf("decision = %v, want %v", got, tc.want)
			}
			if m.run.pending != nil {
				t.Error("the call is still pending after the editor answered it")
			}
			if surface.tool != "run_command" || surface.input != string(input) {
				t.Errorf("editor asked about %q %q, want the call verbatim", surface.tool, surface.input)
			}
			if surface.id == "" {
				t.Error("the approval was published with no id to answer it by")
			}
		})
	}
}

// An editor that was asked and never answered is a refusal, and the deadline is
// what makes that true rather than a hope: Approve returns Refused when its own
// timeout fires, and that is all this loop needs to deny the call. The timeout
// is a constant, so the test asserts the deadline the editor is handed and the
// answer a refusal produces, instead of waiting two minutes for it.
func TestAnEditorApprovalFailsClosedOnTimeout(t *testing.T) {
	surface := &fakeSurface{answer: IDERefused}
	m := attached(surface)
	decision := make(chan approval.Decision, 1)

	cmd := m.parkApproval(approval.Request{Name: "run_command", Decision: decision})
	if cmd == nil {
		t.Fatal("the attached editor was never asked")
	}
	m.Update(cmd())

	if got := <-decision; got != approval.Denied {
		t.Errorf("decision = %v, want the call refused when the editor answers nothing", got)
	}
	if surface.deadline.IsZero() {
		t.Fatal("the editor was asked with no deadline, so a silent one parks the call forever")
	}
	if left := time.Until(surface.deadline); left <= 0 || left > ideApprovalTimeout {
		t.Errorf("deadline in %v, want it inside the %v timeout", left, ideApprovalTimeout)
	}
}

// A late answer is dropped rather than applied to whatever is pending now: the
// reader may have answered the call it names at the terminal, and the call that
// arrived since is a different decision.
func TestALateEditorAnswerDoesNotDecideTheNextCall(t *testing.T) {
	surface := &fakeSurface{}
	m := attached(surface)
	first := make(chan approval.Decision, 1)

	m.parkApproval(approval.Request{Name: "run_command", Decision: first})
	answered := m.ide.asked
	m.key(tea.KeyPressMsg{Code: 'y'})
	if got := <-first; got != approval.AllowedOnce {
		t.Fatalf("terminal decision = %v, want the keypress to have decided", got)
	}

	second := make(chan approval.Decision, 1)
	m.parkApproval(approval.Request{Name: "run_command", Decision: second})
	m.Update(ideEvent{kind: ideAnswer, id: answered, decision: IDEAllowed})

	select {
	case got := <-second:
		t.Errorf("a stale answer decided the next call as %v", got)
	default:
	}
}

func TestNoEditorLeavesTheApprovalToTheTerminal(t *testing.T) {
	m := sized()
	decision := make(chan approval.Decision, 1)

	cmd := m.parkApproval(approval.Request{Name: "search", Decision: decision})
	if cmd != nil {
		t.Error("a session with no editor attached started one anyway")
	}
	if m.run.pending == nil {
		t.Fatal("the request was not parked for the terminal")
	}
	if handled, _ := m.key(tea.KeyPressMsg{Code: 'y'}); !handled {
		t.Fatal("the terminal could not answer the approval")
	}
	if got := <-decision; got != approval.AllowedOnce {
		t.Errorf("decision = %v, want the keypress to decide", got)
	}
}

// A surface with nobody attached is not a refusal. bulle's terminal prompt is
// the reader's approval surface, and a session that opened a socket no editor
// has dialled must leave the call to it: reading "nobody was there to ask" as
// "no" would deny every tool call the moment --ide or a stray BULLE_IDE is set,
// with the prompt still on screen asking a question that is already answered.
func TestAnUnattachedEditorLeavesTheApprovalToTheTerminal(t *testing.T) {
	surface := &fakeSurface{answer: IDEUnanswered}
	m := attached(surface)
	decision := make(chan approval.Decision, 1)

	cmd := m.parkApproval(approval.Request{Name: "run_command", Decision: decision})
	if cmd == nil {
		t.Fatal("the surface was never asked")
	}
	m.Update(cmd())

	select {
	case got := <-decision:
		t.Fatalf("the call was decided as %v by an editor that was never attached", got)
	default:
	}
	if m.run.pending == nil {
		t.Fatal("the call was unparked, so the terminal can no longer answer it")
	}
	if handled, _ := m.key(tea.KeyPressMsg{Code: 'y'}); !handled {
		t.Fatal("the terminal could not answer the call the editor left to it")
	}
	if got := <-decision; got != approval.AllowedOnce {
		t.Errorf("decision = %v, want the keypress to decide", got)
	}
}
