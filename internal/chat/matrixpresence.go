package chat

import (
	"context"
	"errors"
	"time"

	"maunium.net/go/mautrix/id"
)

// Seen sends a read receipt for the message that triggered the run.
func (m *Matrix) Seen(ctx context.Context, to Identity) error {
	if m.client == nil || to.Room == "" || to.Event == "" {
		return errors.New("matrix: no event to mark as seen")
	}
	return m.client.MarkRead(ctx, id.RoomID(to.Room), id.EventID(to.Event))
}

// Typing updates the typing indicator for the given room.
func (m *Matrix) Typing(ctx context.Context, to Identity, typing bool) error {
	if m.client == nil {
		return errors.New("matrix: client not initialized")
	}
	if to.Room == "" {
		return errors.New("matrix: no room for typing indicator")
	}
	timeout := 20 * time.Second
	if !typing {
		timeout = 0
	}
	_, err := m.client.UserTyping(ctx, id.RoomID(to.Room), typing, timeout)
	return err
}
