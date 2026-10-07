package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/nacelle"
)

func stuck() *model {
	m := sized()
	m.run.cancel, m.run.busy = func() {}, true
	return m
}

func TestASecondCtrlCQuitsARunThatWillNotStop(t *testing.T) {
	press := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	m := stuck()

	if handled, cmd := m.key(press); !handled || cmd != nil {
		t.Fatalf("first ctrl+c = %v, %v; want the run cancelled and the session kept", handled, cmd)
	}
	if _, cmd := m.key(press); cmd == nil {
		t.Fatal("a second ctrl+c did not quit, so the only way out is kill -9")
	}
}

func TestTheStatusLineOffersTheEscapeAfterTheFirstCtrlC(t *testing.T) {
	m := stuck()

	if status := m.status(); strings.Contains(status, "ctrl+c") {
		t.Fatalf("status = %q, want no offer before anything was pressed", status)
	}

	m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	status := m.status()
	if !strings.Contains(status, "ctrl+c") || !strings.Contains(status, "quit") {
		t.Errorf("status = %q, want it to say a second ctrl+c quits", status)
	}
}

func TestCtrlBackslashQuitsWhateverTheRunIsDoing(t *testing.T) {
	press := tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl}
	if press.String() != "ctrl+\\" {
		t.Fatalf("constructed key = %q, want ctrl+backslash", press.String())
	}

	m := stuck()
	handled, cmd := m.key(press)
	if !handled || cmd == nil {
		t.Errorf("ctrl+backslash = %v, %v; want an immediate quit", handled, cmd)
	}
}

func TestAnAbandonedRunSaysSoRatherThanReadingAsReady(t *testing.T) {
	m := stuck()
	m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m.settle()

	status := m.status()
	if !strings.Contains(status, "abandoned") {
		t.Errorf("status = %q, want the run reported as abandoned", status)
	}
	if strings.Contains(status, "ready") {
		t.Errorf("status = %q, want half an answer not to read as a finished one", status)
	}
}

func TestANewQuestionClearsTheAbandonedMark(t *testing.T) {
	m := stuck()
	m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m.settle()

	m.agent = answering(t)
	m.prompt.SetValue("again please")
	m.ask()
	defer m.run.cancel()

	if m.run.stop != nacelle.Stop("") {
		t.Errorf("stop = %q, want the previous run's reason cleared", m.run.stop)
	}
}

func TestAFreshQuestionTakesTheForceQuitOfferBackDown(t *testing.T) {
	m := stuck()
	m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m.settle()

	m.agent = answering(t)
	m.prompt.SetValue("again please")
	m.ask()
	defer m.run.cancel()

	if handled, cmd := m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !handled || cmd != nil {
		t.Errorf("ctrl+c = %v, %v; want the new run stopped rather than the client quit", handled, cmd)
	}
	if status := m.status(); !strings.Contains(status, "ctrl+c") {
		t.Errorf("status = %q, want the offer made afresh for this run", status)
	}
}

func TestEscapeStopsTheRunAndLeavesTheClientStanding(t *testing.T) {
	m := stuck()

	handled, cmd := m.key(tea.KeyPressMsg{Code: tea.KeyEscape})

	if !handled || cmd != nil {
		t.Fatalf("esc = %v, %v; want the run stopped and the session kept", handled, cmd)
	}
	if m.run.stop != abandoned {
		t.Errorf("stop = %q, want the run marked abandoned", m.run.stop)
	}
	if status := m.status(); !strings.Contains(status, "stopping") {
		t.Errorf("status = %q, want it to say the run is stopping rather than still running", status)
	}
}

func TestASecondEscapeDoesNotKeepTheForceQuitOfferAlive(t *testing.T) {
	m := stuck()
	m.key(tea.KeyPressMsg{Code: tea.KeyEscape})
	armed := m.run.interrupted

	handled, cmd := m.key(tea.KeyPressMsg{Code: tea.KeyEscape})

	if !handled || cmd != nil {
		t.Errorf("second esc = %v, %v; want the press still claimed by the run", handled, cmd)
	}
	if !m.run.interrupted.Equal(armed) {
		t.Error("a second esc re-armed the offer, so holding it holds ctrl+c at quit")
	}
}

func TestEscapeWithNothingRunningBelongsToThePrompt(t *testing.T) {
	m := sized()

	if handled, cmd := m.key(tea.KeyPressMsg{Code: tea.KeyEscape}); handled || cmd != nil {
		t.Errorf("esc while idle = %v, %v; want it left to the prompt", handled, cmd)
	}
}

func TestEscapeClosesTheDropdownBeforeItStopsAnything(t *testing.T) {
	m := stuck()
	m.prompt.SetValue("/")
	m.refreshMenu()
	if !m.menu.Open() {
		t.Fatal("the dropdown did not open, so there is nothing for esc to close first")
	}

	m.key(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.menu.Open() {
		t.Error("the menu stayed open after esc")
	}
	if m.run.stop == abandoned {
		t.Error("esc stopped the run as well as closing the menu, want one thing per press")
	}
}

func TestAnApprovalRequestShowsInTheStatusLine(t *testing.T) {
	m := sized()
	decision := make(chan approval.Decision, 1)

	m.Update(approval.Request{Name: "search_content", Input: []byte(`{"pattern":"x"}`), Decision: decision})

	if m.run.pending == nil {
		t.Fatal("the request did not set run.pending")
	}
	status := m.status()
	if !strings.Contains(status, "search_content") {
		t.Errorf("status = %q, want the tool's name", status)
	}
	if !strings.Contains(status, "y = once") {
		t.Errorf("status = %q, want the key hint", status)
	}
}

func TestPressingAnswersApproval(t *testing.T) {
	cases := []struct {
		key      rune
		decision approval.Decision
	}{
		{'y', approval.AllowedOnce},
		{'a', approval.AllowedForSession},
		{'n', approval.Denied},
	}
	for _, tc := range cases {
		m := sized()
		decision := make(chan approval.Decision, 1)
		m.run.pending = &approval.Request{Name: "search", Decision: decision}
		handled, _ := m.key(tea.KeyPressMsg{Code: tc.key})
		if !handled {
			t.Fatalf("key %c was not handled", tc.key)
		}
		if m.run.pending != nil {
			t.Errorf("pending not cleared for key %c", tc.key)
		}
		if d := <-decision; d != tc.decision {
			t.Errorf("key %c = %v, want %v", tc.key, d, tc.decision)
		}
	}
}

func TestAnyOtherKeyIsSwallowedWhilePending(t *testing.T) {
	m := sized()
	decision := make(chan approval.Decision, 1)
	m.run.pending = &approval.Request{Name: "search", Decision: decision}

	handled, _ := m.key(tea.KeyPressMsg{Code: 'x'})

	if !handled {
		t.Error("an unrelated key was passed through while an approval was pending")
	}
	if m.run.pending == nil {
		t.Error("the pending approval was cleared by a key that was not a decision")
	}
	select {
	case d := <-decision:
		t.Errorf("a decision (%v) was sent for a key that answered nothing", d)
	default:
	}
}

func TestCtrlCClearsAPendingApprovalAndCancelsTheRun(t *testing.T) {
	m := sized()
	cancelled := false
	m.run.busy = true
	m.run.cancel = func() { cancelled = true }
	decision := make(chan approval.Decision, 1)
	m.run.pending = &approval.Request{Name: "search", Decision: decision}

	handled, cmd := m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if !handled || cmd != nil {
		t.Fatalf("ctrl+c = %v, %v; want the run cancelled and the session kept", handled, cmd)
	}
	if !cancelled {
		t.Error("the run was not cancelled")
	}
	if m.run.pending != nil {
		t.Error("run.pending survived a cancel — the status line would keep asking a dead question")
	}
}
