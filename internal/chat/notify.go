package chat

import "context"

// Notifier is an optional interface for an adapter that can send a quiet
// message and report the id the platform gave a sent message. Both return the
// id of the first chunk, which is what a reaction later points at.
type Notifier interface {
	SendQuiet(ctx context.Context, to Identity, text string) (string, error)
	SendEvent(ctx context.Context, to Identity, text string) (string, error)
}

// Deliver sends text and returns the id of the sent event, empty when the
// adapter cannot report one. A message with notify false goes out quiet where
// the adapter supports it, and as a normal message otherwise.
func Deliver(ctx context.Context, a Adapter, to Identity, text string, notify bool) (string, error) {
	n, ok := a.(Notifier)
	if !ok {
		return "", a.Send(ctx, to, text)
	}
	if notify {
		return n.SendEvent(ctx, to, text)
	}
	return n.SendQuiet(ctx, to, text)
}
