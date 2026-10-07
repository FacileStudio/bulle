package tui

// Tests for the pre-send compaction guard: when it fires, and its refusal to go
// quiet when the backend cannot count. What the guard measures is next door, in
// compact_measure_test.go.
//
// The guard used to be one condition — `if count, err := CountTokens(...); err
// == nil && ...` — with a failure mode invisible from the call site: on a backend
// that reports token counting as Unsupported, err is never nil and the whole
// check is skipped. The post-turn trigger still ran, so compaction looked
// healthy while a conversation already past the ceiling was sent anyway, and the
// overshoot was absorbed into the next turn rather than prevented.

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// blind is a backend shaped like the OpenAI-compatible runner: it runs, and it
// refuses to count. Capabilities.TokenCounting is false and CountTokens returns
// the Unsupported error, which is the measure the pre-send guard has to survive.
type blind struct{}

func (blind) Name() string                       { return "blind" }
func (blind) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{} }

func (blind) CountTokens(context.Context, nacelle.Request) (int64, error) {
	return 0, &nacelle.Unsupported{Backend: "blind", Feature: "token counting"}
}

func (blind) Stream(context.Context, nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

// counting is the opposite: a backend that answers with a real count, so the
// preference for a live measure over a remembered one can be told from the
// fallback.
type counting struct{ tokens int64 }

func (counting) Name() string { return "counting" }

func (counting) Capabilities() nacelle.Capabilities {
	return nacelle.Capabilities{TokenCounting: true}
}

func (c counting) CountTokens(context.Context, nacelle.Request) (int64, error) {
	return c.tokens, nil
}

func (counting) Stream(context.Context, nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

func agentOver(t *testing.T, backend nacelle.Backend) *nacelle.Agent {
	t.Helper()

	agent, err := nacelle.New(nacelle.Config{Backend: backend, System: "be quiet"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return agent
}

// heavyHistory is a conversation whose weight sits in the middle: old turns
// carrying nearly all of the context with a small kept tail. It is the shape a
// summarizer can actually land under the ceiling, which is what decides whether
// a pass spends an LLM call or falls back to a tombstone.
func heavyHistory() []nacelle.Message {
	old := func(id string, n int) nacelle.Message {
		return nacelle.Message{Role: nacelle.RoleUser, Parts: []nacelle.Part{
			nacelle.ToolResult{ID: id, Name: "read", Result: strings.Repeat("x", n)},
		}}
	}
	answered := func() nacelle.Message {
		return nacelle.Message{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "ok"}}}
	}

	conv := []nacelle.Message{{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "goal"}}}}
	for i := range 12 {
		conv = append(conv, old(fmt.Sprintf("old-%d", i), 40_000), answered())
	}
	return append(conv, old("newest", 10), answered())
}

// drain lets a pass the guard started finish. A pass reports on its own channel,
// which in a session the update loop reads and in a test nobody does — left
// alone the goroutine would sit blocked on the send until the test binary exits.
func drain(t *testing.T, m *Model) {
	t.Helper()

	channel := m.run.compactChan
	if channel == nil {
		return
	}
	select {
	case <-channel:
	case <-time.After(10 * time.Second):
		t.Errorf("the pass never reported an outcome")
	}
}

// The guard still fires on the soft tier, which is synchronous and free, and it
// acts on the size it fell back to.
func TestCompactBeforeSendTombstonesWhenTheBackendCannotCount(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, blind{})
	m.engine().Policy.Window = 200_000
	m.conversation = bigConversation()
	m.size = 150_000

	m.compactBeforeSend(context.Background())

	if m.last.tier != compaction.Soft || m.last.results == 0 {
		t.Fatalf("last = %+v, want a tombstone pass earned by the last reported size", m.last)
	}
	if m.size >= 150_000 {
		t.Errorf("size = %d, want the pass to have freed context", m.size)
	}
}

// The same over-threshold conversation on a backend that cannot count still
// starts a real pass, and the send waits on it. This is the case the defect
// swallowed: a heavy turn's overshoot was absorbed because the guard read the
// Unsupported error as "nothing to do".
func TestCompactBeforeSendStartsAPassWhenTheBackendCannotCount(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, blind{})
	m.conversation = heavyHistory()
	m.size = 130_000

	if m.compactBeforeSend(context.Background()) == nil {
		t.Fatal("compactBeforeSend = nil, want the send held behind a pass")
	}
	if !m.compacting {
		t.Error("compacting = false, want a pass in flight")
	}
	drain(t, m)
}

// Under the trigger plus the headroom one turn is expected to need, the send is
// left alone: the guard is for the conversation already past the point of no
// return, not for every send. The comparison is the trigger itself and not a
// slack above it — the trigger is already the soft ratio of the window with the
// answer's reserve held back underneath, so waiting buys no headroom.
func TestCompactBeforeSendLeavesAnAffordableSendAlone(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, blind{})
	m.engine().Policy.Window = 200_000
	m.conversation = bigConversation()
	m.size = m.engine().Policy.Trigger()

	m.compactBeforeSend(context.Background())

	if m.last.results != 0 || m.compacting {
		t.Errorf("last = %+v, compacting = %v, want a send at the trigger left alone", m.last, m.compacting)
	}
}

// One token past it and the guard fires, so "at the trigger" is the boundary and
// not an accidental headroom the old slack used to provide.
func TestCompactBeforeSendFiresOneTokenPastTheTrigger(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, blind{})
	m.engine().Policy.Window = 200_000
	m.conversation = bigConversation()
	m.size = m.engine().Policy.Trigger() + 1

	m.compactBeforeSend(context.Background())

	if m.last.results == 0 {
		t.Errorf("last = %+v, want the guard to fire past the trigger", m.last)
	}
}

// A backed-off session gets no pre-send pass either: the guard shares the thrash
// flag with the post-turn trigger, so one "not now" holds for both.
func TestCompactBeforeSendStandsDownWhenThrashed(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, blind{})
	m.conversation = heavyHistory()
	m.size = 130_000
	m.engine().SetThrash(thrashLimit)

	if cmd := m.compactBeforeSend(context.Background()); cmd != nil {
		t.Error("compactBeforeSend = a pass, want the automatic guard backed off")
	}
}

// compact_at: 0 turns compaction off, and the guard must not count tokens to
// discover that.
func TestCompactBeforeSendStandsDownWhenCompactionIsOff(t *testing.T) {
	m := sized()
	m.agent = agentOver(t, counting{tokens: 1_000_000})
	m.conversation = heavyHistory()
	m.compactAt = 0
	m.size = 130_000

	if cmd := m.compactBeforeSend(context.Background()); cmd != nil {
		t.Error("compactBeforeSend = a pass, want compaction off to mean off")
	}
	if m.compacting {
		t.Error("compacting = true, want no pass started")
	}
}
