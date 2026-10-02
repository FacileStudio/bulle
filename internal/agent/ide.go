package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/FacileStudio/bulle/internal/ide"
	"github.com/FacileStudio/bulle/internal/tui"
)

// ideAdapter hands a started IDE socket to the terminal as the surface it
// drives. The two Commands interfaces are the same three methods declared
// twice — the terminal declares its own so that it does not import this
// package's — so the conversion between them is a cast, and the socket and the
// prompt loop that answers it share no type.
type ideAdapter struct {
	srv *ide.Server
}

// Approve asks the attached editors to answer one pending tool call, and says
// when there was nobody to ask — that answer belongs to the terminal prompt,
// not to this socket.
func (a ideAdapter) Approve(ctx context.Context, id, tool, input string) tui.IDEDecision {
	switch a.srv.Approve(ctx, id, tool, input) {
	case ide.Allowed:
		return tui.IDEAllowed
	case ide.Refused:
		return tui.IDERefused
	default:
		return tui.IDEUnanswered
	}
}

// SetCommands names the handler an editor's commands reach.
func (a ideAdapter) SetCommands(commands tui.Commands) {
	a.srv.SetCommands(ide.Commands(commands))
}

// Turn reports a model turn starting.
func (a ideAdapter) Turn(n int) {
	a.srv.Publish(ide.Turn(n))
}

// Done reports the run over and why.
func (a ideAdapter) Done(reason string, cost float64) {
	a.srv.Publish(ide.Done(reason, cost))
}

// Close takes the socket and its discovery file down with the session.
func (a ideAdapter) Close() error {
	return a.srv.Close()
}

// closeIDE closes the editor surface a session opened, if it opened one. A
// session that published its socket is a session that has to take it down again:
// the socket and its discovery file outlive the process otherwise, and the next
// reader that checks the pid finds a file naming a session that is gone.
func closeIDE(sess *tui.UISession) {
	closer, ok := sess.IDE.(interface{ Close() error })
	if !ok {
		return
	}
	if err := closer.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// ideSurface is the started socket as the surface the terminal drives, or
// nothing when the session publishes to no editor. The nil check is what keeps
// those two apart: a nil *ide.Server handed over inside an interface is not a
// nil surface, it is an attached editor whose every answer is a refusal, and
// that would turn a session's approvals over to nobody.
func ideSurface(srv *ide.Server) tui.IDESurface {
	if srv == nil {
		return nil
	}
	return ideAdapter{srv: srv}
}
