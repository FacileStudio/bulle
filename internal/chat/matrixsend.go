package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// maxEventBytes is the cap a homeserver enforces on one event's canonical
// JSON, signatures and keys included. Nothing in the library chunks for you,
// so an answer past it is refused with M_TOO_LARGE and the person who asked
// sees silence rather than a truncated reply.
const maxEventBytes = 65536

// sendChunkBytes halves that budget for the body alone. Encryption is what
// spends the other half: the ciphertext is base64, which inflates by a third,
// and the event carries device keys and a signature beside it.
const sendChunkBytes = maxEventBytes / 2

// Send delivers one reply, encrypted when the room is, which it is: the crypto
// helper sits on the client, so the message event picks the encrypted path on
// its own. An identity from another adapter is refused rather than guessed at,
// because a wrong room in the right homeserver is still a wrong room.
//
// A long answer goes out as several messages. The alternative is one event the
// homeserver rejects, which loses the whole reply rather than splitting it.
func (m *Matrix) Send(ctx context.Context, to Identity, text string) error {
	if to.Adapter != "matrix" {
		return fmt.Errorf("matrix: refusing to send to a %q identity", to.Adapter)
	}
	if to.Room == "" {
		return errors.New("matrix: no room to send to")
	}
	m.ensureRoom(ctx, id.RoomID(to.Room))
	for _, chunk := range chunks(text, sendChunkBytes) {
		if _, err := m.client.SendMessageEvent(ctx, id.RoomID(to.Room), event.EventMessage, replyTo(to, chunk)); err != nil {
			return err
		}
		to.Event = ""
	}
	return nil
}

// ensureRoom verifies the room's encryption state and warms the membership
// cache in the state store before sending, so outbound group sessions are
// shared with all room members.
func (m *Matrix) ensureRoom(ctx context.Context, roomID id.RoomID) {
	if m.client == nil || m.client.StateStore == nil {
		return
	}
	if !m.isRoomEncrypted(ctx, roomID) {
		return
	}
	m.warmRoomMembers(ctx, roomID)
}

func (m *Matrix) isRoomEncrypted(ctx context.Context, roomID id.RoomID) bool {
	encrypted, err := m.client.StateStore.IsEncrypted(ctx, roomID)
	if err == nil && encrypted {
		return true
	}
	var enc event.EncryptionEventContent
	err = m.client.StateEvent(ctx, roomID, event.StateEncryption, "", &enc)
	return err == nil && enc.Algorithm == id.AlgorithmMegolmV1
}

func (m *Matrix) warmRoomMembers(ctx context.Context, roomID id.RoomID) {
	members, err := m.client.StateStore.GetRoomJoinedOrInvitedMembers(ctx, roomID)
	if err != nil || len(members) > 0 {
		return
	}
	if _, err := m.client.JoinedMembers(ctx, roomID); err != nil {
		report("matrix", fmt.Errorf("caching members for %s: %w", roomID, err))
	}
}

// replyTo builds the message content, threaded onto the question it answers.
//
// The relation is set through the typed content and not a hand-built map so
// that m.relates_to lands in the cleartext half of an encrypted event, which
// is where a server reads it to aggregate the thread; and so the mention list
// carries the sender, which is what a client renders a reply as.
//
// Only the first chunk is threaded. Every later chunk clears the target, since
// a split answer is one reply and attaching four relations to one question
// reads as four separate answers.
func replyTo(to Identity, text string) *event.MessageEventContent {
	content := &event.MessageEventContent{MsgType: event.MsgText, Body: text}
	if to.Event == "" {
		return content
	}
	content.SetReply(&event.Event{ID: id.EventID(to.Event), Sender: id.UserID(to.Sender)})
	return content
}

// chunks splits text into pieces no longer than limit bytes, dropping nothing
// and reordering nothing. Text within the limit is returned whole, so the
// ordinary reply is one message and the splitting is invisible.
func chunks(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	out := []string{}
	for len(text) > limit {
		cut := cutPoint(text, limit)
		out = append(out, text[:cut])
		text = text[cut:]
	}
	return append(out, text)
}

// cutPoint picks where to break, preferring the last line break so a chunk
// ends on a boundary a human wrote rather than mid-sentence. Failing that it
// backs off to a rune boundary: a cut inside a multi-byte character sends a
// replacement character, and prose in most languages is multi-byte.
//
// A line break found within the window is always a safe cut, because newline
// is ASCII and cannot be a continuation byte.
func cutPoint(s string, limit int) int {
	if idx := strings.LastIndexByte(s[:limit], '\n'); idx > 0 {
		return idx + 1
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	if cut == 0 {
		return limit
	}
	return cut
}
