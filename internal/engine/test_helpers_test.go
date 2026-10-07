package engine

import (
	"context"
	"iter"
	"testing"

	"github.com/FacileStudio/nacelle"
)

type fakeBackend struct {
	events []nacelle.Event
	err    error
}

func (f *fakeBackend) Name() string {
	return "fake"
}

func (f *fakeBackend) Capabilities() nacelle.Capabilities {
	return nacelle.Capabilities{}
}

func (f *fakeBackend) CountTokens(context.Context, nacelle.Request) (int64, error) {
	return 0, nil
}

func (f *fakeBackend) Stream(ctx context.Context, _ nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		for _, ev := range f.events {
			if ctx.Err() != nil {
				return
			}
			if !yield(ev, nil) {
				return
			}
		}
		if f.err != nil {
			yield(nacelle.Event{}, f.err)
		}
	}
}

func newTestAgent(t *testing.T, b nacelle.Backend) *nacelle.Agent {
	t.Helper()
	agent, err := nacelle.New(nacelle.Config{Backend: b, System: "test"})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	return agent
}

func collectEvents(ch <-chan Event) []Event {
	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}
