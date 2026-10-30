package cmd

import (
	"sync"

	"github.com/FacileStudio/bulle/internal/chat"
)

type nudgeLog struct {
	mu        sync.Mutex
	reactions *chat.ReactionRouter
	last      map[string][]string
}

func newNudgeLog(reactions *chat.ReactionRouter) *nudgeLog {
	return &nudgeLog{reactions: reactions, last: map[string][]string{}}
}

func (n *nudgeLog) note(room, eventID string, keys []string) {
	if len(keys) == 0 {
		return
	}
	n.mu.Lock()
	n.last[room] = keys
	n.mu.Unlock()
	if n.reactions != nil && eventID != "" {
		n.reactions.Register(eventID, keys)
	}
}

func (n *nudgeLog) lastKeys(room string) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.last[room]
}
