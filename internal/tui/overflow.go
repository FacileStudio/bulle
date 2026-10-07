// Package tui — answering a provider's context-length rejection: detect it,
// promise it, compact, and start the run again once.
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/bulle/internal/herdr"
)

// armRecovery notes a context-length rejection for settle to answer, and reports
// whether it did. It is called from the middle of a run, when the only honest
// options are to fail loudly or to end quietly and let settle try to rescue the
// turn — and ending quietly is why this exists rather than recovery living in
// consume: the run's channel has to close and its state be tidied before a pass
// can start, because the pass snapshots the conversation the run was reading.
//
// It refuses four ways, and each refusal is a case where the failure the reader
// is about to see is the truthful one. A different error is not this failure; a
// turn that already spent its retry has been answered once and a second pass
// would only repeat it; with no agent there is no summarizer to pass the context
// to; and with compaction switched off the reader has asked the client not to
// manage the window for them, so the rejection stands as the provider's own
// answer.
func (m *Model) armRecovery(err error) bool {
	if !engine.DetectOverflow(err) || m.run.overflowTried || m.agent == nil || m.compactAt <= 0 {
		return false
	}
	m.run.overflow = err
	return true
}

// recoverOverflow answers a rejection settle found waiting: it says what it is
// about to do, and hands the turn to a retry. Nothing to compact is one case it
// cannot fix — a conversation with no history zone is a single oversized turn,
// and no pass may touch the anchor or the live window — so the provider's own
// words are said there and the recovery stops.
//
// A turn the reader stopped is the other, and the check has to live here rather
// than in afterRun. An abandoned run is not owed an answer: esc said the turn was
// called off, and retrying it would spend a pass and a run on a question nobody
// is waiting for, against a context the reader has already moved past. maybeGrind
// reads the same signal for the same reason. The rejection above is consumed
// whether or not the retry happens, so a later run cannot find it still armed and
// answer the abandoned turn after all — which is exactly what skipping the call
// from afterRun would leave behind.
func (m *Model) recoverOverflow() tea.Cmd {
	if m.run.overflow == nil {
		return nil
	}
	err := m.run.overflow
	m.run.overflow = nil
	m.run.overflowTried = true

	if m.run.stop == abandoned {
		return nil
	}
	if _, _, ok := compaction.HistoryRange(m.plan()); !ok {
		m.say(fromFailure, err.Error())
		return nil
	}
	m.say(fromCompact, "the model refused the context as too long — compacting and retrying once")
	return m.retryRun()
}

// retryRun starts a run on the conversation exactly as it stands, with no new
// turn: send's bookkeeping without send's question. The fresh context matters —
// settle cancelled the failed run's — and the run is marked busy before the pass
// starts, because settleCompaction is what starts the run a pass was waiting for
// and it only does that for a session that is still running.
//
// The pass is forced rather than chosen. A size the ladder would have caught
// cannot be what just failed: the provider refused the request as sent, whatever
// the last usage figure said, so re-deriving a tier from that figure can pick a
// softer pass than the failure has already proven is needed.
func (m *Model) retryRun() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.run.cancel = cancel
	m.run.bgCtx = ctx
	m.run.busy = true
	m.run.reported = false
	m.run.stop = ""

	herdr.Report(m.herdrClient, herdr.Working)

	if cmd := m.beginCompaction(ctx, true); cmd != nil {
		return cmd
	}
	return m.startRun(ctx)
}
