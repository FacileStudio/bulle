package tui

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

func TestMaskFloorDropsOnlyHistoryResults(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000

	m.maskHistory(m.plan())

	dropped, kept := 0, 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			result, ok := part.(nacelle.ToolResult)
			if !ok {
				continue
			}
			if strings.HasPrefix(result.Result, droppedNotice) {
				dropped++
				continue
			}
			if len(result.Result) > compactMinResult {
				kept++
			}
		}
	}
	if dropped != 1 || kept != 2 {
		t.Errorf("dropped %d and kept %d large results, want the one history result dropped and the anchor and active ones kept", dropped, kept)
	}
	if m.trimmed != 1 {
		t.Errorf("trimmed count = %d, want 1", m.trimmed)
	}
}

func TestMaskKeepsThePairingShape(t *testing.T) {
	m := sized()
	before := bigConversation()
	m.conversation = before
	m.size = compactAt + 1

	m.maskHistory(m.plan())

	for i, message := range m.conversation {
		if len(message.Parts) != len(before[i].Parts) {
			t.Fatalf("message %d changed shape: %d parts, was %d", i, len(message.Parts), len(before[i].Parts))
		}
		for j, part := range message.Parts {
			was, ok := before[i].Parts[j].(nacelle.ToolResult)
			now, still := part.(nacelle.ToolResult)
			if ok != still {
				t.Fatalf("message %d part %d changed kind", i, j)
			}
			if ok && now.ID != was.ID {
				t.Errorf("message %d part %d: id %q, was %q — the pairing broke", i, j, now.ID, was.ID)
			}
		}
	}
}

func TestMaskLeavesAShortConversationAlone(t *testing.T) {
	m := sized()
	m.conversation = []nacelle.Message{nacelle.UserText("too short to have history")}
	m.size = compactAt + 1

	if stats := m.maskHistory(m.plan()); stats != (compaction.MicroStats{}) {
		t.Errorf("stats = %+v, want nothing tombstoned with no history", stats)
	}
	if m.trimmed != 0 {
		t.Errorf("a conversation with no history lost %d results", m.trimmed)
	}
}

func TestMaskIsNotPaidTwiceOnASecondPass(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 1

	m.maskHistory(m.plan())
	first := m.trimmed
	m.size = compactAt + compactSlack + 1

	m.maskHistory(m.plan())

	if m.trimmed != first {
		t.Errorf("second pass trimmed %d more; placeholders were counted as savings again", m.trimmed-first)
	}
}

// thinkingConversation is a history whose assistant turns each carry reasoning,
// returned with the index and text of every one of them so a test can assert on
// them after a pass.
func thinkingConversation() (*Model, map[int]string) {
	msg := func(role nacelle.Role, parts ...nacelle.Part) nacelle.Message {
		return nacelle.Message{Role: role, Parts: parts}
	}
	thought := func(n int) nacelle.Reasoning {
		return nacelle.Reasoning{Text: strings.Repeat("thinking about this ", n)}
	}
	m := sized()
	m.conversation = []nacelle.Message{
		msg(nacelle.RoleUser, nacelle.ToolResult{ID: "a", Name: "read", Result: strings.Repeat("x", 2000)}),
		msg(nacelle.RoleAssistant, thought(5000), nacelle.Text{Text: "old conclusion"}),
		msg(nacelle.RoleUser, nacelle.ToolResult{ID: "b", Name: "read", Result: strings.Repeat("x", 2000)}),
		msg(nacelle.RoleAssistant, thought(3000), nacelle.Text{Text: "middle conclusion"}),
		msg(nacelle.RoleUser, nacelle.ToolResult{ID: "c", Name: "read", Result: strings.Repeat("x", 2000)}),
		msg(nacelle.RoleAssistant, thought(100), nacelle.Text{Text: "recent conclusion"}),
	}
	return m, map[int]string{1: thought(5000).Text, 3: thought(3000).Text, 5: thought(100).Text}
}

// Reasoning is displayed but never sent, so the mask leaves it where it is: a stub
// on it would free no context while taking the chain of thought out of a
// transcript the reader can still scroll back to.
func TestMaskLeavesHistoryThinkingAlone(t *testing.T) {
	m, thoughts := thinkingConversation()
	m.size = compactAt + 50_000

	m.maskHistory(m.plan())

	for index, want := range thoughts {
		if got := m.conversation[index].Parts[0].(nacelle.Reasoning); got.Text != want {
			t.Errorf("assistant message %d reasoning was rewritten: %q", index, got.Text)
		}
	}
	verifyAssistantTextPreserved(t, m.conversation)
}

// The report counts what the pass actually took out — the stubbed results — and
// nothing it did not do. Reasoning is the shape that used to be counted as
// savings while freeing nothing.
func TestMaskCountsOnlyTheStubbedResults(t *testing.T) {
	m, _ := thinkingConversation()
	m.size = compactAt + 50_000

	m.maskHistory(m.plan())

	stubbed := 0
	for _, message := range m.conversation {
		for _, part := range message.Parts {
			if result, ok := part.(nacelle.ToolResult); ok && strings.HasPrefix(result.Result, droppedNotice) {
				stubbed++
			}
		}
	}
	if stubbed == 0 {
		t.Fatal("the mask stubbed nothing, so the count below proves nothing")
	}
	if m.trimmed != stubbed {
		t.Errorf("trimmed count = %d, want the %d stubbed results", m.trimmed, stubbed)
	}
}

func TestSizedCountsEveryBilledInputKind(t *testing.T) {
	m := sized()
	m.sized(nacelle.Usage{InputTokens: 1000, CacheReadTokens: 9000, CacheCreationTokens: 500})
	if m.size != 10_500 {
		t.Errorf("size = %d, want cache reads and creations billed as input", m.size)
	}
}

func TestLedgerCarriesTheSentinel(t *testing.T) {
	block := compaction.BuildLedger("", "Decisions:\n- ship it")
	if !compaction.IsLedger(block) {
		t.Errorf("ledger = %+v, want the state-ledger sentinel", block)
	}
}

func verifyAssistantTextPreserved(t *testing.T, messages []nacelle.Message) {
	t.Helper()
	for i := 1; i < len(messages); i += 2 {
		found := false
		for _, part := range messages[i].Parts {
			if _, ok := part.(nacelle.Text); ok {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("assistant message %d lost its Text part after compact", i)
		}
	}
}
