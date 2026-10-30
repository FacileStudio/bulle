package chat

import (
	"context"
	"sync"
)

type turns struct {
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
}

func newTurns() *turns {
	return &turns{cancel: map[string]context.CancelFunc{}}
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
	cancel, ok := t.cancel[key]
	t.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}
