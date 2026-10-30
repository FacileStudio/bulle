package chat

import (
	"strings"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// messageFrom turns one synced Matrix event into an inbound chat message, or
// reports that it is not one. It is a pure function on purpose: every rule
// that decides whether a room may reach a shell is then testable without a
// homeserver, which is the only cheap way to prove the refusals hold.
//
// The bot's own messages are refused first, because a bot that answers itself
// answers itself forever. Only m.room.message events are read: a reaction or a
// membership change is noise that never arrives here, but the type is asserted
// anyway so a future caller cannot feed one in.
func messageFrom(evt *event.Event, self id.UserID) (Message, bool) {
	if evt == nil || evt.Sender == self || evt.Type != event.EventMessage {
		return Message{}, false
	}
	content := evt.Content.AsMessage()
	if media, ok := audioOf(content); ok {
		return Message{Identity: identityOf(evt), Kind: KindAudio, Media: media, At: time.UnixMilli(evt.Timestamp)}, true
	}
	text, ok := textOf(content)
	if !ok {
		return Message{}, false
	}
	return Message{Identity: identityOf(evt), Text: text, At: time.UnixMilli(evt.Timestamp)}, true
}

func identityOf(evt *event.Event) Identity {
	return Identity{
		Adapter: "matrix",
		Room:    evt.RoomID.String(),
		Sender:  evt.Sender.String(),
		Event:   evt.ID.String(),
	}
}

// textOf is the body of a message a human meant as an instruction, or false.
// A media message carries no instruction a text reply can act on, so it is
// refused rather than answered with its filename. An edit is the same
// instruction with a new body, and answering a corrected prompt twice would
// run the corrected tool twice.
func textOf(content *event.MessageEventContent) (string, bool) {
	if content.MsgType != event.MsgText && content.MsgType != event.MsgNotice {
		return "", false
	}
	if isEdit(content.RelatesTo) {
		return "", false
	}
	if strings.TrimSpace(content.Body) == "" {
		return "", false
	}
	return content.Body, true
}

// isEdit reports whether a message replaces an earlier one. An edit arrives as
// m.replace with the new body under m.new_content, so the replacement ID and
// its relation type both mark it, and a nil relation is simply not an edit.
func isEdit(rel *event.RelatesTo) bool {
	return rel != nil && (rel.Type == event.RelReplace || rel.GetReplaceID() != "")
}
