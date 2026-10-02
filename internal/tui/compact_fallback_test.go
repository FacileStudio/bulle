package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

func TestMaskFallbackKeepsTheConversationStanding(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000

	if !m.applyMaskFallback(compactOutcome{before: m.size, plan: m.plan(), tier: compaction.Smart}) {
		t.Fatal("the mask refused a plan that covers the conversation")
	}

	saved := 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			result, ok := part.(nacelle.ToolResult)
			if ok && strings.HasPrefix(result.Result, droppedNotice) {
				saved++
			}
		}
	}
	if saved == 0 {
		t.Error("mask fallback masked nothing")
	}
	if len(m.conversation) != len(bigConversation()) {
		t.Errorf("mask fallback changed the message count: %d, was %d", len(m.conversation), len(bigConversation()))
	}
	if said := strings.Join(spoken(m), " "); !strings.Contains(said, "✂ Compaction summary") {
		t.Errorf("mask fallback did not report: %v", spoken(m))
	}
}

// The mask refuses a plan that no longer covers the conversation, the same as
// the fold does, and the notice it feeds says nothing landed rather than
// promising a masking that did not happen: tombstoning whatever now sits at
// those indices would be an edit to a conversation the pass never measured.
func TestMaskFallbackRefusesAStalePlan(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	before := m.size
	gone := []nacelle.Message{nacelle.UserText("a conversation that is gone"), nacelle.AssistantText("and its answer")}

	if m.applyMaskFallback(compactOutcome{before: before, plan: compaction.Plan(gone, m.policy), tier: compaction.Smart}) {
		t.Error("the mask ran against a plan that does not cover the conversation")
	}
	if m.trimmed != 0 || m.size != before {
		t.Errorf("a stale mask changed the session: trimmed = %d, size = %d", m.trimmed, m.size)
	}
	if note := maskNote(false); !strings.Contains(note, "nothing was masked") {
		t.Errorf("maskNote(false) = %q, want it to report that no fallback landed", note)
	}
	if note := maskNote(true); !strings.Contains(note, "masked instead") {
		t.Errorf("maskNote(true) = %q, want the fallback reported", note)
	}
}

func TestSettleCompactionFallsBackToTheMaskOnFailure(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	outcome := compactOutcome{before: m.size, plan: m.plan(), tier: compaction.Smart, err: errors.New("summarizer hiccuped")}

	m.settleCompaction(outcome)

	if m.compacting {
		t.Error("compacting still true after the fallback")
	}
	if len(m.conversation) != len(bigConversation()) {
		t.Errorf("mask fallback changed the message count: %d", len(m.conversation))
	}
	saved := 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			result, ok := part.(nacelle.ToolResult)
			if ok && strings.HasPrefix(result.Result, droppedNotice) {
				saved++
			}
		}
	}
	if saved == 0 {
		t.Error("a failed summary bought no headroom: nothing was masked")
	}
	if said := strings.Join(spoken(m), " "); !strings.Contains(said, "compaction summary failed") {
		t.Errorf("failure not reported: %v", said)
	}
}
