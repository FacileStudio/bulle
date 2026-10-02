// Package tui — how a compaction pass presents itself while it runs, and what it
// tells the reader when there was nothing to install: the spinner must keep
// ticking, and the running-row elapsed timer with it, even on the idle and
// /compact paths where run.busy stays false; and a pass that got no usable
// summary must say so rather than leave only a weak mask card.
package tui

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

func TestSpinnerKeepsTickingWhileCompacting(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.compacting = true
	msg, ok := m.spin.Tick().(spinner.TickMsg)
	if !ok {
		t.Fatal("Tick did not produce a spinner.TickMsg")
	}
	if cmd := m.spun(msg); cmd == nil {
		t.Error("the spinner stopped re-arming during a compaction pass (the timer would freeze)")
	}
}

func TestCompactionArmsTheSpinnerAndStampsTheTimer(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	if cmd := m.beginCompaction(context.Background(), false); cmd == nil {
		t.Fatal("beginCompaction returned no command, want the spinner-armed wait")
	}
	defer func() { m.compacting = false }()
	if !m.compacting {
		t.Error("beginCompaction did not mark the session as compacting")
	}
	if m.compactBegan.IsZero() {
		t.Error("beginCompaction did not stamp when the pass began")
	}
	if row := strings.Join(m.inFlightGroups(), "\n"); !strings.Contains(row, "compacting session") {
		t.Errorf("in-flight row = %q, want the compaction row drawn live", row)
	} else if !strings.Contains(row, " · ") {
		t.Errorf("in-flight row = %q, want the elapsed timer after the spinner", row)
	}
}

func TestSettleCompactionSaysWhenTheSummaryCameBackEmpty(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	outcome := compactOutcome{before: int64(125_000), plan: m.plan()}
	m.settleCompaction(outcome)
	if m.compacting {
		t.Errorf("compacting still true after the empty fallback")
	}
	joined := strings.Join(spoken(m), "\n")
	if !strings.Contains(joined, "compaction summary came back empty") {
		t.Errorf("spoken = %v, want the empty-summary note rather than a silent fallback", spoken(m))
	}
}

// A judged pass that tagged turns for the ledger and then came back with an empty
// summary must mask rather than install. Installing it would drop those turns and
// build no ledger, so the reader would lose — silently, and reported as a
// successful summary — exactly what the pass was meant to keep in compressed form.
func TestSettleCompactionMasksWhenAJudgedPassHasNoSummary(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	plan := m.plan()
	blocks := compaction.Blocks(m.conversation, plan)
	if len(blocks) == 0 {
		t.Fatal("the sample produced no history blocks to fold")
	}
	outcome := compactOutcome{
		before: int64(125_000),
		plan:   plan,
		tier:   compaction.Smart,
		judged: true,
		fold:   compaction.Fold{Ledger: blocks},
	}

	m.settleCompaction(outcome)

	if installedLedger(m.conversation) {
		t.Error("a ledger was installed from an empty summary")
	}
	if len(m.conversation) != len(bigConversation()) {
		t.Errorf("conversation = %d messages, want an empty judged pass to mask rather than fold", len(m.conversation))
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "compaction summary came back empty") {
		t.Errorf("said = %q, want the empty-summary note rather than a silent fold", said)
	}
}

// A judged pass that folds nothing worth folding is refused rather than installed:
// rebuilding around a ledger longer than the turns it replaces would grow the very
// context the pass exists to shrink, so the conversation is left as it was and the
// reader is told.
func TestSettleCompactionReportsARefusedFold(t *testing.T) {
	m := sized()
	m.conversation = []nacelle.Message{
		nacelle.UserText("the task"),
		nacelle.AssistantText("ok"),
		nacelle.UserText("u2"),
		nacelle.AssistantText("a2"),
	}
	before := len(m.conversation)

	m.settleCompaction(compactOutcome{
		before:  5,
		plan:    m.plan(),
		tier:    compaction.Smart,
		judged:  true,
		summary: strings.Repeat("decision, constraint, dead end. ", 400),
	})

	if len(m.conversation) != before {
		t.Errorf("conversation = %d messages, want the refused pass to leave it alone", len(m.conversation))
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "context unchanged") {
		t.Errorf("said = %q, want the refused fold named", said)
	}
}
