package chat

import (
	"context"
	"sync"
	"time"
)

// Typer is an optional interface an adapter can implement to indicate typing.
type Typer interface {
	Typing(ctx context.Context, to Identity, typing bool) error
}

// Reader is an optional interface an adapter can implement to mark a message
// as seen the moment it is accepted, before the run starts.
type Reader interface {
	Seen(ctx context.Context, to Identity) error
}

func markSeen(ctx context.Context, a Adapter, to Identity) {
	reader, ok := a.(Reader)
	if !ok {
		return
	}
	if err := reader.Seen(ctx, to); err != nil {
		report(to.Adapter, err)
	}
}

func startTyping(ctx context.Context, a Adapter, to Identity) func() {
	typer, ok := a.(Typer)
	if !ok {
		return func() {}
	}
	tryTyping(ctx, typer, to, true)
	done := make(chan struct{})
	var once sync.Once
	go runTypingLoop(ctx, typer, to, done)
	return func() {
		once.Do(func() {
			close(done)
			stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			tryTyping(stopCtx, typer, to, false)
		})
	}
}

func runTypingLoop(ctx context.Context, typer Typer, to Identity, done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			tryTyping(ctx, typer, to, true)
		}
	}
}

func tryTyping(ctx context.Context, typer Typer, to Identity, typing bool) {
	if err := typer.Typing(ctx, to, typing); err != nil {
		report(to.Adapter, err)
	}
}
