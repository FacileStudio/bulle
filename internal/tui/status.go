package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/cost"
	"github.com/FacileStudio/bulle/internal/layout"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/nacelle"
)

const abandoned nacelle.Stop = "abandoned"

func (m *Model) status() string {
	isReady := true
	state := "✓ ready"
	if cut := cutShort(m.run.stop); cut != "" {
		state = cut
		isReady = false
	}
	if m.run.busy {
		state = m.working()
		isReady = false
		if time.Since(m.run.interrupted) < forceQuit {
			state = "stopping · ctrl+c or ctrl+\\ to quit now"
		}
	}
	if m.run.pending != nil {
		state = fmt.Sprintf("approve %s(%s)? y = once · a = always this session · n = deny",
			m.run.pending.Name, layout.Truncate(layout.Unstyled(string(m.run.pending.Input)), 60))
	}

	if m.session != nil && m.session.HasWriteError() {
		state = "! could not write to session log · " + state
		isReady = false
	}

	width := max(m.width, 1)
	counts := strings.Join(m.footer(), " ")
	stateLine := layout.Truncate(state, width)
	if isReady {
		stateLine = m.theme.Ready.Render(stateLine)
	}
	return stateLine + "\n" + m.theme.Muted.Render(layout.Truncate(counts, width))
}

// footer is the stats line under the state line: the provider and model being
// billed, in the same words the launch banner uses and following a /model
// switch, then the price when the backend reported one, the input and output
// tokens, and the context size. The output tokens, the context and the price
// all carry the live estimate (liveOut) so they tick as the model writes — the
// price scaled by the last realised cost per token (rate) — and the real
// per-turn usage replaces the estimate the moment a turn ends. A session with a
// grind budget adds what is left of the current run's minimum, measured against
// that run alone, and a session without one looks exactly as it did before.
func (m *Model) footer() []string {
	var spent []string
	label := m.activeBackend
	if m.activeModel != "" {
		if label != "" {
			label += " · "
		}
		label += m.activeModel
	}
	if label != "" {
		spent = append(spent, label)
	}

	total := m.total()
	total.OutputTokens += m.run.liveOut
	total.Cost += m.rate * float64(m.run.liveOut)

	if total.Cost > 0 {
		spent = append(spent, cost.FormatCost(total.Cost))
	}
	spent = append(spent, tokenTotals(total))
	if m.size > 0 {
		spent = append(spent, contextLoad(m.size+m.run.liveOut, m.engine().Policy))
	}
	run := m.run.usage
	run.OutputTokens += m.run.liveOut
	run.Cost += m.rate * float64(m.run.liveOut)
	if left := m.grind.left(run); left != "" {
		spent = append(spent, "grind "+left)
	}
	return spent
}

func (m *Model) working() string {
	if m.compacting {
		return m.theme.Compacting.Render(m.spin.View() + " compacting session")
	}
	doing := status.WaitingVerb(time.Since(m.run.began))
	switch n := m.running(); n {
	case 0:
	case 1:
		name, ok := m.runningName()
		if ok {
			doing = "running " + name
		}
	default:
		doing = fmt.Sprintf("running %d tools", n)
	}
	if since := m.ongoing(); since != "" {
		doing += " · " + since
	}
	return yellow.Render(m.spin.View() + " " + doing)
}

func (m *Model) running() int {
	n := 0
	for _, g := range m.run.groups {
		if g.End.IsZero() {
			n++
		}
	}
	return n
}

func (m *Model) runningName() (string, bool) {
	for _, g := range m.run.groups {
		if g.End.IsZero() {
			return g.Tool.Name, true
		}
	}
	return "", false
}

func (m *Model) ongoing() string {
	if !m.run.busy || m.run.began.IsZero() {
		return ""
	}
	return status.Lasted(time.Since(m.run.began))
}

// spun advances the spinner. A compaction pass keeps it (and the running-row
// timer that redraws with it) ticking even though run.busy is false on the
// idle and /compact paths — without it that row freezes at the frame and
// elapsed time it had when the pass started.
func (m *Model) spun(message spinner.TickMsg) tea.Cmd {
	return m.spin.Spun(message, m.run.busy || m.hasLiveParallel() || m.compacting)
}

func cutShort(stop nacelle.Stop) string {
	if stop == "" || stop.Complete() {
		return ""
	}
	switch stop {
	case nacelle.StopMaxTokens:
		return "cut off at the token limit"
	case nacelle.StopContext:
		return "cut off: out of context"
	case nacelle.StopRefusal:
		return "refused by the model"
	case nacelle.StopIterations:
		return "stopped at the iteration limit"
	case abandoned:
		return "abandoned"
	}
	return "stopped early"
}
