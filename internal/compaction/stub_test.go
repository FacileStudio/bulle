package compaction

import (
	"context"
	"iter"
	"time"

	"github.com/FacileStudio/nacelle"
)

type summarizingBackend struct{ answer string }

func (summarizingBackend) Name() string { return "summarizing" }

func (summarizingBackend) Capabilities() nacelle.Capabilities {
	return nacelle.Capabilities{Effort: true}
}

func (summarizingBackend) CountTokens(context.Context, nacelle.Request) (int64, error) {
	return 0, nil
}

func (s summarizingBackend) Stream(_ context.Context, _ nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		if !yield(nacelle.Event{Kind: nacelle.KindText, Text: s.answer}, nil) {
			return
		}
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

type blockingBackend struct{}

func (blockingBackend) Name() string { return "blocking" }

func (blockingBackend) Capabilities() nacelle.Capabilities {
	return nacelle.Capabilities{Effort: true}
}

func (blockingBackend) CountTokens(context.Context, nacelle.Request) (int64, error) {
	return 0, nil
}

func (blockingBackend) Stream(ctx context.Context, _ nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		for ctx.Err() == nil {
			time.Sleep(1 * time.Millisecond)
		}
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}
