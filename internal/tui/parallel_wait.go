package tui

import (
	"strconv"

	"github.com/FacileStudio/bulle/internal/sessions"
)

// relaxAfterDispatch ends the parent run once a model-callable parallel
// fan-out has registered its batch. Detach already made the tool non-blocking;
// this is the other half — the model must not keep working a turn whose
// results it cannot read, because a detached fan-out's outcomes stream to
// PostDetached and never back to its stream. Cancelling closes the stream,
// settle runs, and the prompt returns to ready while the spinner rows keep
// ticking under it.
func (m *Model) relaxAfterDispatch() {
	if !m.run.busy {
		return
	}
	m.run.cancel()
}

// announceStart commits the green fan-out confirmation. No transcript speaker
// is green, so it renders the Ready style directly rather than through paint:
// one line, logged as client text, with a check-mark prefix. A singular task is
// one "agent"; more than one is "agents".
func (m *Model) announceStart(n int) {
	noun := "parallel agents"
	if n == 1 {
		noun = "parallel agent"
	}
	msg := "✓ " + strconv.Itoa(n) + " " + noun + " started"
	m.unprinted = append(m.unprinted, m.theme.Ready.Render(msg))
	m.session.Line(sessions.Speaker(fromClient), msg)
}
