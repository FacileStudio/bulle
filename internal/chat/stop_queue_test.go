package chat

import (
	"context"
	"sync"
	"testing"
	"time"
)

type queuedAdapter struct {
	fakeAdapter
	started chan struct{}
}

func (q *queuedAdapter) Receive(_ context.Context, handle func(Message)) error {
	handle(q.in[0])
	<-q.started
	handle(q.in[1])
	handle(q.in[2])
	return nil
}

func TestStopDropsQueuedMessages(t *testing.T) {
	a := &queuedAdapter{started: make(chan struct{})}
	a.in = []Message{
		msg("!r:x", "@alice:x", "long job"),
		msg("!r:x", "@alice:x", "queued behind"),
		msg("!r:x", "@alice:x", "/stop"),
	}
	var mu sync.Mutex
	var seen []string
	respond := func(ctx context.Context, m Message) (string, error) {
		mu.Lock()
		seen = append(seen, m.Text)
		mu.Unlock()
		if m.Text == "long job" {
			close(a.started)
			<-ctx.Done()
			return "", ctx.Err()
		}
		return "ran " + m.Text, nil
	}
	done := make(chan error, 1)
	go func() {
		done <- RunWith(context.Background(), a, respond, Router{Allow: []string{"@alice:x"}}, &Commands{})
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not finish")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || seen[0] != "long job" {
		t.Fatalf("turns run = %v, want only the stopped one", seen)
	}
}
