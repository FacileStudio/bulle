package tui

// Tests for runCompaction, the pass goroutine that turns a summarizer call
// into a compactOutcome. The happy path and the deadline path are each driven
// on a real agent built over a stub backend, so the timeout behaviour is
// exercised without a network.

import (
	"context"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// blocking is a backend whose stream stalls until its context is cancelled —
// the shape of a hung endpoint — and then winds down cleanly without reporting
// an error, as a transport that honours cancellation by stopping does. A pass
// that hits this must still read the deadline off the context and fail rather
// than install whatever partial text happened to stream in.
type blocking struct{}

func (blocking) Name() string                       { return "blocking" }
func (blocking) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{Effort: true} }

func (blocking) CountTokens(context.Context, nacelle.Request) (int64, error) { return 0, nil }

func (blocking) Stream(ctx context.Context, _ nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		for ctx.Err() == nil {
			time.Sleep(1 * time.Millisecond)
		}
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

// summarizing is a backend that answers with one text blob, the happy path a
// summarizer takes.
type summarizing struct{ answer string }

func (summarizing) Name() string                       { return "summarizing" }
func (summarizing) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{Effort: true} }

func (summarizing) CountTokens(context.Context, nacelle.Request) (int64, error) { return 0, nil }

func (s summarizing) Stream(_ context.Context, _ nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		if !yield(nacelle.Event{Kind: nacelle.KindText, Text: s.answer}, nil) {
			return
		}
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

// summarizerAgent builds a real agent on a test backend, so a test can drive
// runCompaction's summarizer path (which needs m.agent set) without a network.
func summarizerAgent(t *testing.T, b nacelle.Backend) *nacelle.Agent {
	t.Helper()
	agent, err := nacelle.New(nacelle.Config{Backend: b, System: "compact"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return agent
}

func TestRunCompactionCollectsASummary(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.agent = summarizerAgent(t, summarizing{answer: "Decisions:\n- done."})
	results := make(chan compactOutcome)

	go runCompaction(t.Context(), results, m.pass(m.plan(), compaction.Smart, false))

	outcome, open := <-results
	if !open {
		t.Fatalf("compaction channel closed before an outcome arrived")
	}
	if outcome.err != nil {
		t.Fatalf("err = %v, want a clean summary", outcome.err)
	}
	if !strings.Contains(outcome.summary, "Decisions:") {
		t.Errorf("summary = %q, want the streamed text collected", outcome.summary)
	}
}

func TestRunCompactionFallsBackWhenTheSummarizerDeadlineFires(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	m.agent = summarizerAgent(t, blocking{})
	results := make(chan compactOutcome)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	go runCompaction(ctx, results, m.pass(m.plan(), compaction.Smart, false))

	outcome, open := <-results
	if !open {
		t.Fatalf("compaction channel closed before an outcome arrived")
	}
	if outcome.err == nil {
		t.Fatalf("err = %v, want the deadline read as a failure", outcome.err)
	}
	if outcome.summary != "" {
		t.Errorf("summary = %q, want the partial text dropped once the deadline fired", outcome.summary)
	}
}
