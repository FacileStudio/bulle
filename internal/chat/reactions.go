package chat

import (
	"context"
	"sync"
)

const reactionRouterSize = 500

// ReactionRouter remembers which item keys a sent event carried, so a reaction
// on that event can be routed back to them. It keeps the newest 500 events and
// is safe for concurrent use. A nil router maps nothing.
type ReactionRouter struct {
	mu    sync.Mutex
	order []string
	keys  map[string][]string
}

// NewReactionRouter returns an empty router.
func NewReactionRouter() *ReactionRouter {
	return &ReactionRouter{keys: map[string][]string{}}
}

// Register maps a sent event id to item keys, evicting the oldest event once
// the router is full. An empty id or an empty key list is ignored.
func (r *ReactionRouter) Register(eventID string, keys []string) {
	if r == nil || eventID == "" || len(keys) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, seen := r.keys[eventID]; !seen {
		r.order = append(r.order, eventID)
	}
	r.keys[eventID] = append([]string(nil), keys...)
	for len(r.order) > reactionRouterSize {
		delete(r.keys, r.order[0])
		r.order = r.order[1:]
	}
}

// Lookup returns the item keys registered for an event id, or nil.
func (r *ReactionRouter) Lookup(eventID string) []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.keys[eventID]...)
}

func (c *Commands) react(ctx context.Context, m Message) {
	if c == nil || c.OnReaction == nil {
		return
	}
	if keys := c.Reactions.Lookup(m.Target); len(keys) > 0 {
		c.OnReaction(ctx, m, keys)
	}
}
