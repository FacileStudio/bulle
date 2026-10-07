package tui

// Tests for the recovery half of the pre-send guard: the ladder is measured
// against an estimate, so a provider can still refuse a request it thinks is too
// long. The answer is the one the provider asked for — compact, then send again
// — and the whole design question is how to do that once and only once.

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// errTooLong is a provider's own refusal, in the shape Anthropic prints it.
var errTooLong = errors.New("prompt is too long: 213000 tokens > 200000 maximum")

// overflowing is a session whose run just died of that refusal: the conversation
// has a heavy history to fold and the run is still on the books.
func overflowing(t *testing.T) *Model {
	t.Helper()

	m := sized()
	m.agent = agentOver(t, silent{})
	m.conversation = heavyHistory()
	m.size = 130_000
	m.run.busy = true
	m.run.cancel = func() {}
	m.run.overflow = errTooLong
	return m
}

// The rejection is held rather than printed, because settle is the one place that
// knows whether a pass can still rescue the turn — and a red failure the client
// is about to undo is worse than no line at all.
func TestAContextRejectionIsHeldRatherThanReported(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, silent{})
	m.conversation = heavyHistory()
	m.size = 130_000

	m.consume(result{err: errTooLong})

	if m.run.overflow == nil {
		t.Fatal("run.overflow = nil, want the rejection held for settle")
	}
	if said := strings.Join(spoken(m), "\n"); said != "" {
		t.Errorf("said = %q, want the rejection kept quiet", said)
	}
}

// An ordinary failure still fails loudly: the recovery answers one provider
// vocabulary and nothing else.
func TestAnOrdinaryFailureIsStillReported(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, silent{})

	m.consume(result{err: errors.New("429 rate limit")})

	if m.run.overflow != nil {
		t.Error("run.overflow set, want only context-length rejections held")
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "429 rate limit") {
		t.Errorf("said = %q, want the failure reported", said)
	}
}

// The retry is spent once per turn, which is what stops a provider that refuses
// even the compacted conversation from looping through passes forever.
func TestASecondRejectionIsNotHeldForRetry(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, silent{})
	m.run.overflowTried = true

	if m.armRecovery(errTooLong) {
		t.Error("armRecovery = true, want the turn's one retry already spent")
	}
}

// Retrying with compaction switched off is not a recovery: the reader has asked
// the client not to manage the window, so the provider's answer is the answer.
func TestARejectionWithCompactionOffIsNotHeld(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, silent{})
	m.compactAt = 0

	if m.armRecovery(errTooLong) {
		t.Error("armRecovery = true, want compaction off to mean off")
	}
}

// Settle is where the retry happens: the pass is started, the run is marked busy
// so the pass's own settle starts the run it was waiting for, and the attempt is
// recorded as spent.
func TestSettleCompactsAndRetriesOnceAfterARejection(t *testing.T) {
	m := overflowing(t)

	if cmd := m.settle(); cmd == nil {
		t.Fatal("settle = nil, want the recovery's pass")
	}
	if !m.compacting {
		t.Error("compacting = false, want a pass in flight")
	}
	if !m.run.busy {
		t.Error("busy = false, want the run still on the books for the pass to restart")
	}
	if m.run.overflow != nil || !m.run.overflowTried {
		t.Errorf("overflow = %v, tried = %v, want it answered", m.run.overflow, m.run.overflowTried)
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "compacting and retrying once") {
		t.Errorf("said = %q, want the retry promised", said)
	}
	drain(t, m)
}

// Nothing to compact is the one rejection recovery cannot fix: a conversation
// with no history zone is a single oversized turn, and no pass may touch the
// anchor or the live window. The provider's own words are said instead.
func TestARejectionWithNothingToCompactReportsTheProviderWords(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, silent{})
	m.conversation = []nacelle.Message{nacelle.UserText("one enormous paste")}
	m.run.overflow = errTooLong

	if cmd := m.recoverOverflow(); cmd != nil {
		t.Error("recoverOverflow = work, want nothing to do without a history zone")
	}
	if !m.run.overflowTried {
		t.Error("overflowTried = false, want the attempt to count even when it cannot help")
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "prompt is too long") {
		t.Errorf("said = %q, want the provider's own words", said)
	}
}

// A turn the reader stopped is not owed an answer. esc marks the run abandoned
// before settle reaches the recovery, and a retry there would spend a pass and a
// run on a question that was called off — answering a turn nobody is waiting for,
// against a context they have already moved past. The rejection is consumed
// rather than left armed, which is the half a guard in afterRun would have
// missed: an armed error found later answers the abandoned turn after all.
func TestARejectionOnAnAbandonedTurnIsNeitherRetriedNorLeftArmed(t *testing.T) {
	m := overflowing(t)
	m.run.stop = abandoned

	if cmd := m.recoverOverflow(); cmd != nil {
		t.Error("recoverOverflow = work, want an abandoned turn left alone")
	}
	if m.run.overflow != nil {
		t.Errorf("overflow = %v, want the rejection consumed rather than left for a later run", m.run.overflow)
	}
	if !m.run.overflowTried {
		t.Error("overflowTried = false, want the attempt recorded even though it was not made")
	}
	if m.compacting {
		t.Error("compacting = true, want no pass started for a turn the reader stopped")
	}
	if said := strings.Join(spoken(m), "\n"); said != "" {
		t.Errorf("said = %q, want nothing said about a turn that was abandoned", said)
	}
}

// The guard is the abort and nothing else: the same fixture, un-abandoned, still
// recovers, so it cannot be read as recovery that quietly stopped happening.
func TestARejectionOnATurnNobodyStoppedStillRecovers(t *testing.T) {
	m := overflowing(t)

	if cmd := m.recoverOverflow(); cmd == nil {
		t.Fatal("recoverOverflow = nil, want the retry for a turn nobody stopped")
	}
	if !m.compacting {
		t.Error("compacting = false, want the pass the retry asked for")
	}
	drain(t, m)
}

// The retry forces the fold. It is the one caller that knows better than the
// estimate: a provider refused the request as sent, so whatever the ladder last
// measured was already wrong.
func TestARetryForcesTheFold(t *testing.T) {
	m := overflowing(t)
	m.size = m.engine().Policy.Trigger() + 1
	m.engine().Judge = stubJudge{}

	if cmd := m.retryRun(); cmd == nil {
		t.Fatal("retryRun = nil, want a pass before the run starts again")
	}
	outcome := <-m.run.compactChan

	for i := range m.conversation {
		if outcome.Fold.Survives(i) {
			t.Errorf("index %d survives the retry's fold, want the whole history folded", i)
		}
	}
}

// The two ways to force are independent, and this pins the explicit one. With a
// trigger the gentle fold comfortably clears, the derived lever stays its hand —
// so a keep-all judge leaves every block in place — and only the flag the retry
// carries can empty the fold. Without this, `pass.force` could be deleted and the
// suite would not notice, because every other fixture also trips the derivation.
func TestARetryForcesEvenWhenTheEstimateSaysTheFoldWouldLand(t *testing.T) {
	m := overflowing(t)
	m.engine().Judge = stubJudge{}
	m.engine().Policy.Ceiling = 10_000_000
	m.compactAt = m.engine().Policy.Ceiling
	m.size = 130_000

	plan := m.plan()
	keeps := compaction.Fold{Kept: compaction.Blocks(m.conversation, plan)}
	if !compaction.LandsUnder(m.conversation, plan, keeps, m.engine().Policy.Trigger()) {
		t.Fatal("the fixture's gentle fold does not land, so the derived lever would fire and the flag is not being tested")
	}

	if cmd := m.retryRun(); cmd == nil {
		t.Fatal("retryRun = nil, want a pass before the run starts again")
	}
	outcome := <-m.run.compactChan

	for i := range m.conversation {
		if outcome.Fold.Survives(i) {
			t.Errorf("index %d survives, want the retry's flag to fold the history the estimate said it could keep", i)
		}
	}
}
