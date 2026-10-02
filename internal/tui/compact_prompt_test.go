package tui

// Tests for the ask a pass puts in front of the summarizer, and for the one
// property the design calls the ledger's protection: a call that may rewrite the
// ledger is never handed the ledger alone. I3b in docs/plan-compaction-robust.md
// is the reason a rewrite is legitimate at all — it is measured against turns no
// earlier pass compressed, rather than being a summary of a summary — so what is
// asserted here is checked against the call runCompaction actually makes, not
// against the builder in isolation.

import (
	"context"
	"iter"
	"reflect"
	"slices"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// recorder is a backend that keeps the request it was last asked to run, so a
// test can read back exactly what the summarizer was handed. It answers with a
// body that keeps the fixture's load-bearing identifier, the shape a rewrite that
// installs has.
type recorder struct{ last nacelle.Request }

func (*recorder) Name() string                       { return "recorder" }
func (*recorder) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{} }

func (*recorder) CountTokens(context.Context, nacelle.Request) (int64, error) { return 0, nil }

func (r *recorder) Stream(_ context.Context, req nacelle.Request) iter.Seq2[nacelle.Event, error] {
	r.last = req
	return func(yield func(nacelle.Event, error) bool) {
		if !yield(nacelle.Event{Kind: nacelle.KindText, Text: "Decisions:\n- keep " + ledgerIdentifier}, nil) {
			return
		}
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

// rawTurns is I3b's counter: how many of the messages a call was handed carry a
// turn as the conversation holds it, byte for byte, with no summary and no stub
// standing in for it. A call carrying the earlier ledger and a few of these is a
// rewrite measured against fresh material; a call carrying the ledger alone has
// nothing to measure against, which is the shape the invariant refuses.
//
// The turns are matched by prefix rather than by equality because the ask rides
// in the last of them: withAsk folds it into the final history turn when that
// turn is already the user's, so the message that goes out is the raw turn with
// the ask's text appended. That is still an uncompressed turn, and a counter that
// missed it would report one turn fewer than the call carried.
func rawTurns(sent, history []nacelle.Message) int {
	turns := 0
	for _, msg := range sent {
		if slices.ContainsFunc(history, func(turn nacelle.Message) bool { return carriesTurn(msg, turn) }) {
			turns++
		}
	}
	return turns
}

// carriesTurn reports whether a sent message is a history turn, with the ask's
// own text optionally appended to it.
func carriesTurn(msg, turn nacelle.Message) bool {
	if msg.Role != turn.Role || len(msg.Parts) < len(turn.Parts) {
		return false
	}
	return reflect.DeepEqual(msg.Parts[:len(turn.Parts)], turn.Parts)
}

// rewriteCall makes the call a consolidating pass makes over the fixture and
// returns what the summarizer was handed, the history it should have been handed,
// and the earlier ledger's body — so a test can check the three against each
// other without rebuilding any of them.
func rewriteCall(t *testing.T) (sent, history []nacelle.Message, ledger string) {
	t.Helper()

	backend := &recorder{}
	m := sized()
	m.agent = agentOver(t, backend)
	m.conversation = overdueLedger()
	m.size = 130_000

	plan := m.plan()
	history = compaction.HistoryMessages(m.conversation, plan)
	ledger = compaction.LedgerText(m.conversation, plan)
	if len(history) == 0 || ledger == "" {
		t.Fatal("the fixture carries no turns or no earlier ledger, so no rewrite is being asked for")
	}

	pass := m.pass(plan, compaction.Smart, false)
	if !pass.consolidate {
		t.Fatal("the pass is not consolidating, so the invariant is not being exercised")
	}
	results := make(chan compactOutcome, 1)
	runCompaction(context.Background(), results, pass)
	if outcome := <-results; outcome.summary == "" {
		t.Fatalf("the pass asked for no summary (%v), so there is no ask to check", outcome.err)
	}
	return backend.last.Messages, history, ledger
}

// A rewriting pass carries the earlier ledger in its ask by design, and the turns
// it is rewriting it against in the prompt — the property that makes asking for a
// rewrite honest at all. The pass is run for real, so what is asserted is what the
// summarizer would have been sent, not what a builder says it would build.
func TestARewriteCallIsNeverHandedTheLedgerAlone(t *testing.T) {
	sent, history, ledger := rewriteCall(t)

	if got := rawTurns(sent, history); got != len(history) {
		t.Errorf("the rewrite call carried %d of the %d uncompressed turns, want all of them alongside the ledger", got, len(history))
	}
	if !promptText(sent, ledger[:40]) {
		t.Error("the ask carries no earlier ledger, so the call this test is about was never made")
	}
	if !promptText(sent, "rewritten rather than added to") {
		t.Error("the ask does not ask for a rewrite, so the invariant was not exercised")
	}
}

// The counter has to be able to see the shape the invariant forbids, or the test
// above proves only that it can see something. An ask built from the earlier
// ledger and nothing else — the shape I3b exists to refuse — must count zero
// uncompressed turns.
func TestTheI3bCounterSeesALedgerOnlyAsk(t *testing.T) {
	_, history, ledger := rewriteCall(t)

	ledgerOnly := []nacelle.Message{nacelle.UserText(compactAskWith(ledger, true))}

	if got := rawTurns(ledgerOnly, history); got != 0 {
		t.Errorf("a ledger-only ask counted %d uncompressed turns, so the invariant's own test cannot fail on the shape it refuses", got)
	}
}

// A plan that no longer covers the conversation it was measured against leaves a
// pass with no turn to send, and that is the one shape where carrying the earlier
// ledger would hand it over alone. The ledger stays behind with the ask that
// would have rewritten it: with nothing to measure a rewrite against, there is no
// rewrite to ask for, and the standing ask is what goes out instead.
func TestAPromptWithNoTurnsCarriesNoLedger(t *testing.T) {
	m := sized()
	m.conversation = overdueLedger()
	plan := m.plan()
	past := compaction.Fold{Ledger: []compaction.Block{{
		Key:   "measured against a longer conversation",
		Start: len(m.conversation),
		End:   len(m.conversation) + 4,
	}}}

	ledger := compaction.LedgerText(m.conversation, plan)
	if ledger == "" {
		t.Fatal("the fixture carries no earlier ledger, so this test proves nothing")
	}

	prompt := compactPrompt(m.conversation, plan, past, true)

	if promptText(prompt, ledger[:40]) {
		t.Error("a call with no turn to send still carried the earlier ledger, which is the shape I3b refuses")
	}
	if !promptText(prompt, compactAsk) {
		t.Error("prompt = no standing ask, want the call to fall back to it")
	}
	if promptText(prompt, "rewritten rather than added to") {
		t.Error("prompt asks for a rewrite with nothing to rewrite the ledger against")
	}
}

// The add-only path is the other half of the same rule: a pass that may not
// rewrite still shows the ledger, and still shows it next to the turns it is
// adding to it.
func TestAnAddingCallCarriesTheLedgerNextToItsTurns(t *testing.T) {
	m := sized()
	m.conversation = append(
		[]nacelle.Message{nacelle.UserText("the task"), compaction.BuildLedger("", "Decisions:\n- keep "+ledgerIdentifier)},
		heavyHistory()...,
	)
	plan := m.plan()
	history := compaction.HistoryMessages(m.conversation, plan)
	if len(history) == 0 {
		t.Fatal("the fixture has no history zone, so there is nothing to add to the ledger")
	}

	prompt := compactPrompt(m.conversation, plan, compaction.Fold{Ledger: compaction.Blocks(m.conversation, plan)}, false)

	if got := rawTurns(prompt, history); got != len(history) {
		t.Errorf("the adding call carried %d of the %d turns, want all of them beside the ledger", got, len(history))
	}
	if !promptText(prompt, "do not repeat anything it holds") {
		t.Error("prompt = no earlier ledger, want it carried with the no-restatement rule")
	}
}
