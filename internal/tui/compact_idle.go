// Package tui — the triggers of context compaction: the pre-send guard, the
// automatic post-turn check and the manual /compact command.
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/compaction"
)

// compactBeforeSend is the pre-flight half of the trigger: when the context the
// next turn is about to inherit has already crossed the tier trigger, compact
// first and hold the send until the pass settles. It is the guard that turns the
// threshold into prevention — without it a single heavy turn is absorbed rather
// than avoided.
//
// It fires at the trigger itself and not a slack above it. The ceiling it is
// compared against is already the soft ratio of the window a turn can fill,
// with the reserve for the answer held back underneath it, so a conversation
// past that figure has no runway left to spend: waiting twenty thousand tokens
// longer only changes which tier answers, and it changes it upward.
//
// It must not depend on the backend being able to count tokens. Counting is the
// accurate measure when it is offered, because it sees the conversation exactly
// as it will be sent, but the OpenAI-compatible runner reports token counting as
// Unsupported — and a guard that only runs when counting succeeds is a guard
// that silently does nothing on that backend. Counting is therefore the
// preferred measure, not the required one: when it is unavailable the last size
// the provider reported in a usage event stands in. That is the same source the
// post-turn trigger reads, so the two automatic paths agree on what the
// conversation costs instead of one of them going quiet.
func (m *Model) compactBeforeSend(ctx context.Context) tea.Cmd {
	if m.compactAt <= 0 || m.thrashed() {
		return nil
	}
	size, ok := m.sendSize(ctx)
	if !ok || size <= m.engine().Policy.Trigger() {
		return nil
	}
	m.size = size
	return m.compactTiered(ctx)
}

// sendSize reports what the next send would carry. It prefers a real count from
// the backend and falls back to the last usage-reported size. ok is false only
// when neither is available, which is the one case where compaction has nothing
// to measure against and so must not fire on a guess — and a count of zero is
// read as no answer rather than a measure, because a non-empty conversation
// never costs nothing: zero from a backend means it counted nothing, not that
// there is nothing to count.
func (m *Model) sendSize(ctx context.Context) (int64, bool) {
	if m.agent != nil {
		if count, err := m.agent.CountTokens(ctx, m.conversation); err == nil && count > 0 {
			return count, true
		}
	}
	if m.size > 0 {
		return m.size, true
	}
	return 0, false
}

// shouldCompactIdle is the decision behind maybeCompactIdle, split out so a
// test can drive it without running a pass. It asks the questions that have
// stable checkable answers: is no pass already running, is nothing queued that
// will start a run, and is the conversation over the trigger. The absolute
// compact_at is checked before the tier trigger on purpose: an explicit 0 turns
// compaction off, while Policy.Trigger still reports a ratio for the window it
// was resolved against, so the guard is what honours the disable.
func (m *Model) shouldCompactIdle() bool {
	if m.compacting || m.nextToSend() >= 0 || m.thrashed() {
		return false
	}
	return m.compactAt > 0 && m.size > m.engine().Policy.Trigger()
}

// maybeCompactIdle triggers a compaction pass after a run ends, when the model
// is idle and the conversation was left over the threshold, so a turn that
// finished too heavy is not left sitting at an absurd size while the reader
// waits to type. It is the post-turn half of the trigger: send compacts before
// a run that needs the freed context, and this compacts after a turn that grew
// too big — beginning no run, because the pass reports through
// settleCompaction, which starts nothing while the model is idle.
//
// It only fires when nothing is queued, so it cannot race a run about to
// start: deliver hands any queued line to send, whose own pre-flight check sees
// the same context. While the pass runs the update loop waits on it exactly as
// the send path does, so a message typed meanwhile is queued, not sent into a
// compacting middle.
func (m *Model) maybeCompactIdle() tea.Cmd {
	if !m.shouldCompactIdle() {
		return nil
	}
	return m.compactTiered(context.Background())
}

// compactCmd is the manual /compact: run a compaction pass now, on demand,
// whether or not the conversation has crossed the automatic threshold. It is
// the explicit half of the trigger, for when the reader wants the context
// reclaimed at a natural break rather than waiting to overshoot. A pass is an
// LLM call, so it must not race a live run — the command only fires while
// idle, and while the pass runs the reader's next keystroke queues behind it
// (ask treats an in-flight pass like busy). A fresh manual pass clears any
// thrash flag, because asking by hand is a positive re-attempt.
func (m *Model) compactCmd() tea.Cmd {
	if m.compactAt <= 0 {
		m.say(fromClient, "compaction is off — limits.compact_at is 0")
		return nil
	}
	if m.run.busy {
		m.say(fromClient, "finish the current run, then /compact")
		return nil
	}
	if m.compacting {
		m.say(fromClient, "already compacting")
		return nil
	}
	if _, _, ok := compaction.HistoryRange(m.plan()); !ok {
		m.say(fromClient, "nothing to compact — the conversation is too short")
		return nil
	}
	m.engine().ResetThrash()
	return m.beginCompaction(context.Background(), false)
}
