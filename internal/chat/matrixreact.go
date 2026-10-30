package chat

import (
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// reactionFrom turns a synced m.reaction into a reaction message, or reports
// that it is not one. Encrypted rooms need no extra step: the crypto helper
// decrypts m.room.encrypted and dispatches the plaintext event under its real
// type, so a reaction arrives here the way a text message does. The bot's own
// reactions are refused, and so is a reaction with no target or no key.
func reactionFrom(evt *event.Event, self id.UserID) (Message, bool) {
	if evt == nil || evt.Sender == self || evt.Type != event.EventReaction {
		return Message{}, false
	}
	rel := evt.Content.AsReaction().GetRelatesTo()
	if rel.Type != event.RelAnnotation || rel.EventID == "" || rel.Key == "" {
		return Message{}, false
	}
	return Message{
		Identity: identityOf(evt),
		Kind:     KindReaction,
		Reaction: rel.Key,
		Target:   rel.EventID.String(),
		At:       time.UnixMilli(evt.Timestamp),
	}, true
}
