package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/nacelle"
)

// grindBudget is the per-run minimum spend a session can demand before the
// model is allowed to stop: a run that ends under it, on a stop the model
// itself chose, is continued with one notice naming what is left, up to cap
// continuations per run. Both floors at zero means the budget is off and
// none of this ever fires.
type grindBudget struct {
	cost   float64
	tokens int64
	cap    int
	used   int
}

// on reports whether any floor is configured. A budget nobody set is off,
// and a session without one behaves exactly as it did before this existed.
func (g grindBudget) on() bool {
	return g.cost > 0 || g.tokens > 0
}

// owed decides whether a finished run earns a continuation: the budget is
// on, the cap has not been reached, the stop is one the model chose, and the
// run's spend is short of every floor that was set. With both floors
// configured a run owes both — the floors are two units of one minimum, not
// two ways to satisfy it.
func (g grindBudget) owed(stop nacelle.Stop, spend nacelle.Usage) bool {
	return g.on() && g.used < g.cap && grindableStop(stop) && g.short(spend)
}

// short reports whether the spend misses any configured floor.
func (g grindBudget) short(spend nacelle.Usage) bool {
	if g.cost > 0 && spend.Cost < g.cost {
		return true
	}
	return g.tokens > 0 && spend.OutputTokens < g.tokens
}

// grindableStop says whether the model chose this ending. StopEnd covers the
// quiet give-up that arrives as a normal answer ("I cannot do this"), which
// the stop reason never flags on its own; StopRefusal is the explicit one,
// and StopMaxTokens an answer cut off mid-thought — all three a
// continuation can pick back up. Every other ending is the harness or the
// reader deciding: StopContext would fail again the same way, StopIterations
// is MaxIterations doing its job, StopTools never ends a run, and abandoned
// was the person stopping it on purpose.
func grindableStop(stop nacelle.Stop) bool {
	switch stop {
	case nacelle.StopEnd, nacelle.StopRefusal, nacelle.StopMaxTokens:
		return true
	default:
		return false
	}
}

// unspent describes the shortfall: what is left of each configured floor.
func (g grindBudget) unspent(spend nacelle.Usage) string {
	var pieces []string
	if g.cost > 0 {
		pieces = append(pieces, fmt.Sprintf("$%.2f of $%.2f", max(g.cost-spend.Cost, 0), g.cost))
	}
	if g.tokens > 0 {
		pieces = append(pieces, fmt.Sprintf("%s of %s output tokens", status.ShortTokens(max(g.tokens-spend.OutputTokens, 0)), status.ShortTokens(g.tokens)))
	}
	return strings.Join(pieces, " · ")
}

// notice is the continuation message the harness sends as the model's next
// user turn: what is still unspent and what that asks of the model.
func (g grindBudget) notice(spend nacelle.Usage) string {
	return "grind budget: this run is under its minimum spend · " + g.unspent(spend) +
		" unspent · continue working on the task instead of stopping or refusing · " +
		"stop only when the minimum is consumed or the work is genuinely complete"
}

// left is the remaining budget for the footer, empty once every floor is met.
func (g grindBudget) left(spend nacelle.Usage) string {
	var pieces []string
	if g.cost > 0 {
		if remaining := g.cost - spend.Cost; remaining > 0 {
			pieces = append(pieces, fmt.Sprintf("$%.2f", remaining))
		}
	}
	if g.tokens > 0 {
		if remaining := g.tokens - spend.OutputTokens; remaining > 0 {
			pieces = append(pieces, status.ShortTokens(remaining)+" tok")
		}
	}
	return strings.Join(pieces, "/")
}

// maybeGrind continues a run that stopped short of its minimum spend. The
// notice goes to the transcript as well as onto the conversation, so the
// reader can see why the run did not end where the model ended it. used
// counts here rather than inside send, because the continuation itself rides
// send: the counter belongs to the run, not to the send that starts it.
func (m *Model) maybeGrind(spend nacelle.Usage) tea.Cmd {
	if !m.grind.owed(m.run.stop, spend) {
		return nil
	}
	m.grind.used++
	notice := m.grind.notice(spend)
	m.say(fromTool, "⟳ "+notice)
	return m.send(notice)
}
