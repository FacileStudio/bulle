package cmd

import (
	"context"
	"time"

	"github.com/FacileStudio/nacelle"
)

const (
	lockPoll    = 25 * time.Millisecond
	neutralUser = "Notifications du jour."
	maxPending  = 50
)

func (r *chatRoom) lockCtx(ctx context.Context) error {
	for !r.mu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockPoll):
		}
	}
	return nil
}

func (r *chatRoom) record(text string) {
	r.pmu.Lock()
	r.pending = append(r.pending, text)
	if len(r.pending) > maxPending {
		r.pending = r.pending[len(r.pending)-maxPending:]
	}
	r.pmu.Unlock()
	if r.mu.TryLock() {
		defer r.mu.Unlock()
		r.flushPending()
	}
}

func (r *chatRoom) flushPending() {
	r.pmu.Lock()
	texts := r.pending
	r.pending = nil
	r.pmu.Unlock()
	if len(texts) == 0 {
		return
	}
	conv := r.session.Conversation()
	for _, text := range texts {
		if conv.Len() == 0 {
			conv.AppendUserText(neutralUser)
		}
		conv.AppendAssistantText(text)
		conv.CloseTurn()
	}
	r.persist()
}

func userFirst(msgs []nacelle.Message) []nacelle.Message {
	if len(msgs) == 0 || msgs[0].Role == nacelle.RoleUser {
		return msgs
	}
	first := nacelle.Message{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: neutralUser}}}
	return append([]nacelle.Message{first}, msgs...)
}
