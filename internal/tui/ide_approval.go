package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/herdr"
)

// ideApprovalTimeout is how long an attached editor has to answer a pending
// tool call before the session refuses it. The terminal prompt has no clock of
// its own, so this is the only thing that ends a wait on an editor that has
// gone quiet — and what it ends with is a refusal.
const ideApprovalTimeout = 2 * time.Minute

// askEditor asks the attached editor to answer one pending tool call, and
// returns the command that carries its answer back to this loop. Approve blocks
// on a socket, so it runs in a command of its own rather than in the update
// loop, and the loop learns the answer as a message like any other. A session
// whose surface is off pays the nil check and nothing else: the terminal prompt
// it already has is the whole of its approval surface.
//
// A surface with no editor attached answers Unanswered, immediately, and the
// parked call is left to the terminal. That is the difference between opening a
// socket and handing the session's approvals to nobody.
func (m *Model) askEditor(req approval.Request) tea.Cmd {
	surface := m.ide.surface
	if surface == nil {
		return nil
	}
	m.ide.seq++
	id, tool, input := fmt.Sprintf("approval-%d", m.ide.seq), req.Name, string(req.Input)
	m.ide.asked = id
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ideApprovalTimeout)
		defer cancel()
		return ideEvent{kind: ideAnswer, id: id, decision: surface.Approve(ctx, id, tool, input)}
	}
}

// editorAnswer applies an attached editor's answer to the call it names. An
// answer naming a call nobody is waiting on is dropped rather than applied to
// whatever is pending now: the reader may have answered that call at the
// terminal, or stopped the run, and the call that came after it is a different
// decision. An editor's yes allows one call and never the session — nothing in
// the protocol widens it, so nothing here does either.
func (m *Model) editorAnswer(answer ideEvent) {
	pending := m.run.pending
	if pending == nil || answer.id != m.ide.asked {
		return
	}
	if answer.decision == IDEUnanswered {
		return
	}
	m.run.pending = nil
	decision := approval.Denied
	if answer.decision == IDEAllowed {
		decision = approval.AllowedOnce
	}
	pending.Decision <- decision
	herdr.Report(m.herdrClient, herdr.Working)
}
