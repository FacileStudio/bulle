package chat

import (
	"context"
	"sync"
)

type turns struct {
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
	epochs map[string]uint64
}

func newTurns() *turns {
	return &turns{cancel: map[string]context.CancelFunc{}, epochs: map[string]uint64{}}
}

func (t *turns) begin(ctx context.Context, key string) (context.Context, func()) {
	turn, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.cancel[key] = cancel
	t.mu.Unlock()
	return turn, func() {
		t.mu.Lock()
		delete(t.cancel, key)
		t.mu.Unlock()
		cancel()
	}
}

func (t *turns) stop(key string) bool {
	t.mu.Lock()
	t.epochs[key]++
	cancel, ok := t.cancel[key]
	t.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (t *turns) epoch(key string) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.epochs[key]
}
