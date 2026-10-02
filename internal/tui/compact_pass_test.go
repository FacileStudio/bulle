package tui

// Tests for the snapshot one compaction pass runs on: what it owns, and what it
// therefore cannot be reached by. A pass is the one thing in this package that
// reads a conversation off the update loop's goroutine, so the question here is
// whether anything left on that loop can still edit the words it is reading.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// maskingFixture is a session whose mask has real work to do: heavy history, a
// plan with a history zone in it, and bulky tool results for the tombstone to
// replace. A pass that masked nothing would prove nothing about either half.
func maskingFixture(t *testing.T) (*Model, []compaction.Span) {
	t.Helper()

	m := sized()
	m.policy = windowedPolicy()
	m.conversation = heavyHistory()
	m.size = 130_000

	plan := m.plan()
	if _, _, ok := compaction.HistoryRange(plan); !ok {
		t.Fatal("the fixture has no history zone, so there is nothing for a mask to edit")
	}
	return m, plan
}

// The snapshot is the pass's own conversation, and this is the guard on it.
// Tombstone edits a message's Parts array in place — the one edit in this package
// that is not a whole-slice replacement — and it runs on the update loop, so a
// snapshot sharing those arrays would be reading words the mask was rewriting
// under it. The test masks the model the way a caller that never asked whether a
// pass was live would, and checks the pass's own view did not move. Sharing the
// arrays back makes it fail, on the stub and on the array identity both.
func TestTheSnapshotCannotBeReachedByAMask(t *testing.T) {
	m, plan := maskingFixture(t)
	snapshot := m.pass(plan, compaction.Smart, false)

	m.maskHistory(plan)

	if m.trimmed == 0 {
		t.Fatal("the mask stubbed nothing, so this test proves nothing")
	}
	for i, msg := range snapshot.conv {
		for _, part := range msg.Parts {
			result, ok := part.(nacelle.ToolResult)
			if ok && strings.HasPrefix(result.Result, droppedNotice) {
				t.Fatalf("snapshot message %d carries a stub: the pass and the mask share a parts array", i)
			}
		}
		if len(msg.Parts) > 0 && &msg.Parts[0] == &m.conversation[i].Parts[0] {
			t.Errorf("message %d shares its parts array with the model's conversation", i)
		}
	}
}

// filler is a conversation of n messages with two parts each, which is the shape
// the snapshot has to copy: what costs anything is the number of parts, never how
// much text they hold.
func filler(n int) []nacelle.Message {
	conv := make([]nacelle.Message, n)
	for i := range conv {
		role := nacelle.RoleUser
		if i%2 == 1 {
			role = nacelle.RoleAssistant
		}
		conv[i] = nacelle.Message{Role: role, Parts: []nacelle.Part{
			nacelle.Text{Text: strings.Repeat("x", 4000)},
			nacelle.ToolResult{ID: fmt.Sprintf("t-%d", i), Name: "read", Result: strings.Repeat("y", 4000)},
		}}
	}
	return conv
}

// BenchmarkPassSnapshot is the measurement the clone in pass is justified by: a
// parts array copies as a slice of interface headers, so a conversation of 400
// messages carrying eight kilobytes each comes out in tens of microseconds, and
// none of it is the text. Run it before deciding the clone is too expensive for
// the update loop.
func BenchmarkPassSnapshot(b *testing.B) {
	m := sized()
	m.policy = windowedPolicy()
	m.conversation = filler(400)

	for b.Loop() {
		snapshots = snapshot(m.conversation)
	}
}

// snapshots is where the benchmark's result goes, so the clone cannot be
// optimised away as unused.
var snapshots []nacelle.Message
