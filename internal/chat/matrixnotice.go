package chat

import (
	"context"
	"errors"
	"fmt"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var _ Notifier = (*Matrix)(nil)

// SendQuiet sends text as m.notice, which clients do not push by default, and
// returns the id of the first chunk.
func (m *Matrix) SendQuiet(ctx context.Context, to Identity, text string) (string, error) {
	return m.sendAs(ctx, to, text, event.MsgNotice)
}

// SendEvent sends text as m.text and returns the id of the first chunk.
func (m *Matrix) SendEvent(ctx context.Context, to Identity, text string) (string, error) {
	return m.sendAs(ctx, to, text, event.MsgText)
}

func (m *Matrix) sendAs(ctx context.Context, to Identity, text string, msgType event.MessageType) (string, error) {
	if to.Adapter != "matrix" {
		return "", fmt.Errorf("matrix: refusing to send to a %q identity", to.Adapter)
	}
	if to.Room == "" {
		return "", errors.New("matrix: no room to send to")
	}
	m.ensureRoom(ctx, id.RoomID(to.Room))
	first := ""
	for _, chunk := range chunks(text, sendChunkBytes) {
		resp, err := m.client.SendMessageEvent(ctx, id.RoomID(to.Room), event.EventMessage, contentAs(to, chunk, msgType))
		if err != nil {
			return first, err
		}
		if first == "" {
			first = resp.EventID.String()
		}
		to.Event = ""
	}
	return first, nil
}

func contentAs(to Identity, text string, msgType event.MessageType) *event.MessageEventContent {
	content := replyTo(to, text)
	content.MsgType = msgType
	return content
}
