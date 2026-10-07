package tui

// Tests for the light lever, the tier dispatch and the thrash guard.

import (
	"context"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// compactSlack is a fixture offset above the trigger, used to build the band a pass can stop inside.
const compactSlack = 20_000

// windowedPolicy is a policy whose ratios can be measured: soft at 130k and smart
// at 160k of a 200k window, with compact_at pinned at 100k.
func windowedPolicy() compaction.Policy {
	return compaction.Policy{
		Ratios:         compaction.Ratios{Soft: 0.65, Smart: 0.80},
		Window:         200_000,
		Ceiling:        100_000,
		KeepTurns:      3,
		AnchorMessages: 1,
	}
}

func TestEvictionCanLandUnderRequiresTheHistoryToMatter(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	start, end, _ := compaction.HistoryRange(m.plan())

	m.size = int64(1_000_000)
	if m.evictionCanLandUnder(start, end) {
		t.Error("evictionCanLandUnder = true, want false when the pinned ends alone overshoot the ceiling")
	}

	m.size = int64(105_000)
	if !m.evictionCanLandUnder(start, end) {
		t.Error("evictionCanLandUnder = false, want true when folding the history lands a 105k conversation under the ceiling")
	}
}

func TestBeginCompactionSkipsTheSummarizerWhenEvictionCannotLandUnder(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = int64(1_000_000)

	if cmd := m.beginCompaction(context.Background(), false); cmd != nil {
		t.Error("beginCompaction = a Cmd, want nil when the history cannot land under the ceiling — no summarizer call")
	}
	if m.compacting {
		t.Error("compacting = true, want no summarizer goroutine spawned")
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "kept tail") {
		t.Errorf("report = %q, want the kept-tail explanation on a skipped pass", said)
	}
}

func TestMaskOnlyPassReportsTheKeptTailAndCountsTowardThrash(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = int64(1_000_000)

	cmd := m.maskOnlyPass(m.plan())

	if cmd != nil {
		t.Error("maskOnlyPass = a Cmd, want nil")
	}
	if m.engine().ThrashCount() != 1 {
		t.Errorf("thrashCount = %d, want 1 after a skip left the context over the threshold", m.engine().ThrashCount())
	}
	if said := strings.Join(spoken(m), "\n"); !strings.Contains(said, "kept tail") {
		t.Errorf("report = %q, want the kept-tail explanation", said)
	}
}

// The soft tier is free: it tombstones history and returns with no goroutine and
// no model call.
func TestSoftTierTombstonesWithoutAModelCall(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.engine().Policy = windowedPolicy()
	m.size = 140_000

	cmd := m.compactTiered(context.Background())

	if cmd != nil {
		t.Error("compactTiered = a Cmd at the soft tier, want nil so no pass waits")
	}
	if m.compacting {
		t.Error("compacting = true, want no goroutine at the soft tier")
	}
	if m.trimmed == 0 {
		t.Error("trimmed = 0, want the soft tier to have tombstoned the history")
	}
	if said := strings.Join(spoken(m), " "); strings.Contains(said, "summarized") {
		t.Errorf("report = %q, want no summary on a soft pass", said)
	}
}

// The soft tier is free to run but not free of consequences: clearing a history
// result rewrites the messages after it, which is the prefix the provider had
// already cached. Below the clear floor the pass would trade that re-write for a
// few hundred bytes, so it stays quiet instead of trimming on a marginal
// overshoot.
func TestSoftTierSkipsAPassBelowTheClearFloor(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.size = 140_000
	m.conversation = []nacelle.Message{
		nacelle.UserText("the task"),
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{
			nacelle.ToolResult{ID: "small", Name: "read", Result: strings.Repeat("x", compaction.MinCleared-1)},
		}},
		nacelle.AssistantText("an answer"),
		nacelle.UserText("a newer turn"),
		nacelle.AssistantText("the newest turn"),
	}

	cmd := m.compactTiered(context.Background())

	if cmd != nil {
		t.Error("compactTiered = a Cmd, want nil")
	}
	if m.trimmed != 0 {
		t.Errorf("trimmed = %d, want nothing tombstoned under the clear floor", m.trimmed)
	}
	if m.size != 140_000 {
		t.Errorf("size = %d, want it untouched by a pass that did not run", m.size)
	}
	if said := strings.Join(spoken(m), " "); said != "" {
		t.Errorf("said = %q, want no report for a pass that did not run", said)
	}
}

// Just over the floor the same pass does run, so the gate is a floor and not a
// change of policy: the tier still trims as soon as trimming is worth it.
func TestSoftTierRunsOnceThePassClearsTheFloor(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.size = 140_000
	m.conversation = []nacelle.Message{
		nacelle.UserText("the task"),
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{
			nacelle.ToolResult{ID: "big", Name: "read", Result: strings.Repeat("x", compaction.MinCleared)},
		}},
		nacelle.AssistantText("an answer"),
		nacelle.UserText("a newer turn"),
		nacelle.AssistantText("the newest turn"),
	}

	cmd := m.compactTiered(context.Background())

	if cmd != nil {
		t.Error("compactTiered = a Cmd, want nil so no pass waits")
	}
	if m.trimmed != 1 {
		t.Errorf("trimmed = %d, want the one result over the floor tombstoned", m.trimmed)
	}
}

// A pass that leaves the size over the trigger has not landed, whatever margin it
// left, so the threshold counted here is the one that fires the next pass. The band
// that used to read as a landing ran from the trigger to compactSlack above it, and
// a summarizing pass stopping inside it cleared the count instead of raising it, so
// the guard never stood down while a fresh full pass fired on every send.
func TestCheckThrashCountsAPassThatLeavesTheSizeInTheFiringBand(t *testing.T) {
	m := sized()
	m.conversation = []nacelle.Message{
		nacelle.UserText("the goal"),
		nacelle.AssistantText("an early answer"),
		nacelle.UserText("a middle turn"),
		nacelle.UserText("the newest turn"),
		nacelle.AssistantText("the live answer"),
	}
	m.size = m.engine().Policy.Trigger() + compactSlack/2

	for i := range thrashLimit - 1 {
		if cmd := m.beginCompaction(context.Background(), false); cmd != nil {
			t.Fatalf("pass %d = a Cmd, want the mask-only pass of a history that cannot free the overshoot", i+1)
		}
		if m.engine().ThrashCount() != i+1 {
			t.Fatalf("thrashCount = %d after %d passes that did not land, want %d", m.engine().ThrashCount(), i+1, i+1)
		}
		if m.thrashed() {
			t.Errorf("thrashed after %d passes over the trigger, want the guard to wait for %d", i+1, thrashLimit)
		}
		if said := strings.Join(spoken(m), " "); strings.Contains(said, "compaction keeps leaving") {
			t.Errorf("said = %q, want no stand-down warning before the limit", said)
		}
	}

	m.beginCompaction(context.Background(), false)

	if !m.thrashed() {
		t.Errorf("thrashed = false after %d consecutive passes left the size over the trigger, want the guard stood down", thrashLimit)
	}
	if said := strings.Join(spoken(m), " "); !strings.Contains(said, "compaction keeps leaving") {
		t.Errorf("said = %q, want the stand-down warning at the limit", said)
	}
}

func TestCheckThrashResetsTheCounterWhenUnder(t *testing.T) {
	m := sized()
	m.engine().SetThrash(thrashLimit - 1)
	m.size = m.engine().Policy.Trigger() - 1

	m.checkThrash()

	if m.engine().ThrashCount() != 0 {
		t.Errorf("thrashCount = %d after a pass landed under the threshold, want it reset", m.engine().ThrashCount())
	}
	if m.thrashed() {
		t.Error("thrashed = true after a pass landed under, want it cleared")
	}
}

// The pre-send guard acts at the trigger itself, and what it dispatches there has
// to be a pass that can shrink the conversation. On a backend that reports no
// window there is no ratio to measure, so the ceiling is the whole ladder and Tier
// reads Smart at exactly it: the first size the guard acts on, Trigger() + 1, is a
// summarizing pass already.
//
// This drives the guard rather than reading Tier off an assumed firing size. The
// earlier shape asked about Trigger() + compactSlack + 1, a size the guard stopped
// acting at when it moved to the trigger, so it proved nothing about the dispatch.
// A windowed session whose compact_at sits below its own smart ratio fires into the
// soft tier, and that is accepted: a free tombstone with no model call.
func TestThePreSendGuardDispatchesASummarizingPassAtTheTrigger(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, blind{})
	m.conversation = heavyHistory()
	m.size = m.engine().Policy.Trigger() + 1

	if tier := m.engine().Policy.Tier(m.size); tier != compaction.Smart {
		t.Fatalf("tier at the trigger = %s, want smart with no window to measure a ratio against", tier)
	}
	if cmd := m.compactBeforeSend(context.Background()); cmd == nil {
		t.Fatal("compactBeforeSend = nil one token past the trigger, want the send held behind a pass")
	}
	if !m.compacting {
		t.Error("compacting = false, want a summarizing pass rather than a tombstone no ceiling needs")
	}
	drain(t, m)
}
