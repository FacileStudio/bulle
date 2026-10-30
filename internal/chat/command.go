package chat

import (
	"context"
	"strings"
)

// Handler runs one command and returns the reply text. An empty reply sends
// nothing.
type Handler func(ctx context.Context, m Message, args string) string

// Commands is what the daemon does in code, before any model call. A nil field
// leaves that command to the model, so a daemon wires only what it implements.
// /stop always cancels the running turn of the room; Stop only customises the
// reply. Audio answers an audio message, and OnReaction receives a reaction
// whose target was registered in Reactions, with the item keys of that target.
// Intercept sees every text message before the queue and returns true to take
// it, which is how a pending approval reads a reply while its turn is blocked.
type Commands struct {
	Stop       Handler
	New        Handler
	Snooze     Handler
	Quiet      Handler
	Forget     Handler
	Status     Handler
	Audio      Handler
	OnReaction func(ctx context.Context, m Message, keys []string) bool
	Intercept  func(m Message) bool
	Reactions  *ReactionRouter
}

var commandNames = map[string]bool{
	"stop": true, "new": true, "snooze": true, "quiet": true, "forget": true, "status": true,
}

func parseCommand(text string) (name, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	first, _, _ := strings.Cut(text, "\n")
	word := strings.Fields(first)[0]
	name = strings.ToLower(word[1:])
	if !commandNames[name] {
		return "", "", false
	}
	return name, strings.TrimSpace(text[len(word):]), true
}

func (c *Commands) handler(name string) Handler {
	switch name {
	case "stop":
		return c.Stop
	case "new":
		return c.New
	case "snooze":
		return c.Snooze
	case "quiet":
		return c.Quiet
	case "forget":
		return c.Forget
	}
	return c.Status
}

func (c *Commands) match(m Message) (name string, h Handler, args string) {
	if c == nil || !m.Is(KindText) {
		return "", nil, ""
	}
	name, args, ok := parseCommand(m.Text)
	if !ok {
		return "", nil, ""
	}
	return name, c.handler(name), args
}
