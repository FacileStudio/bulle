package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

const approvalWait = 10 * time.Minute

type approvalCard struct {
	event string
	reply chan bool
}

type approvals struct {
	mu        sync.Mutex
	adapter   chat.Adapter
	reactions *chat.ReactionRouter
	pending   map[string][]*approvalCard
	wait      time.Duration
	after     func(time.Duration) <-chan time.Time
}

func newApprovals(adapter chat.Adapter, reactions *chat.ReactionRouter) *approvals {
	return &approvals{
		adapter:   adapter,
		reactions: reactions,
		pending:   map[string][]*approvalCard{},
		wait:      approvalWait,
		after:     time.After,
	}
}

func approvalText(summary string) string {
	return fmt.Sprintf("Je vais exécuter : %s — réponds ✅ ou « oui » pour valider, ❌ ou « non » pour annuler (10 min).", summary)
}

func roomIdentity(key string) chat.Identity {
	adapter, room, _ := strings.Cut(key, ":")
	return chat.Identity{Adapter: adapter, Room: room}
}

// Ask posts a notifying approval card in the room and waits for a verdict. A
// timeout, a cancelled context or a delivery failure denies.
func (a *approvals) Ask(ctx context.Context, room string, summary string) (bool, error) {
	card := &approvalCard{reply: make(chan bool, 1)}
	a.add(room, card)
	defer a.drop(room, card)
	id, err := chat.Deliver(ctx, a.adapter, roomIdentity(room), approvalText(summary), true)
	if err != nil {
		return false, err
	}
	a.bind(room, card, id)
	select {
	case ok := <-card.reply:
		return ok, nil
	case <-a.after(a.wait):
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (a *approvals) add(room string, card *approvalCard) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending[room] = append(a.pending[room], card)
}

func (a *approvals) bind(room string, card *approvalCard, id string) {
	a.mu.Lock()
	card.event = id
	a.mu.Unlock()
	a.reactions.Register(id, []string{"approval:" + room})
}
