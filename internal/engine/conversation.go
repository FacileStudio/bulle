package engine

import (
	"strings"

	"github.com/FacileStudio/nacelle"
)

// Conversation manages the message history and active turn buffers for an agent session.
type Conversation struct {
	messages    []nacelle.Message
	asked       []nacelle.ToolCall
	answered    []nacelle.ToolResult
	pendingText strings.Builder
}

// NewConversation creates a Conversation, optionally initialized with existing messages.
func NewConversation(messages ...nacelle.Message) *Conversation {
	c := &Conversation{}
	if len(messages) > 0 {
		c.messages = alternate(pruneOrphanedCalls(messages))
	}
	return c
}

// Len returns the number of committed messages in the conversation.
func (c *Conversation) Len() int {
	return len(c.Messages())
}

// Messages returns a copy of the conversation messages.
func (c *Conversation) Messages() []nacelle.Message {
	c.CloseResults()
	c.CloseTurn()
	c.messages = alternate(pruneOrphanedCalls(c.messages))
	return cloneMessages(c.messages)
}

// Snapshot returns a copy of the conversation messages.
func (c *Conversation) Snapshot() []nacelle.Message {
	return c.Messages()
}

// AppendUserText appends user text to the conversation.
func (c *Conversation) AppendUserText(text string) {
	if text == "" {
		return
	}
	c.CloseResults()
	c.CloseTurn()
	c.commit(nacelle.RoleUser, []nacelle.Part{nacelle.Text{Text: text}})
}

// AppendAssistantText appends text to the assistant turn buffer.
func (c *Conversation) AppendAssistantText(text string) {
	if text == "" {
		return
	}
	if len(c.answered) > 0 {
		c.CloseResults()
	}
	c.pendingText.WriteString(text)
}

// CloseResults commits buffered tool results as a user message.
func (c *Conversation) CloseResults() {
	if len(c.answered) == 0 {
		return
	}
	parts := c.answeredParts()
	c.answered = nil
	c.commit(nacelle.RoleUser, parts)
}

func cloneMessages(msgs []nacelle.Message) []nacelle.Message {
	out := make([]nacelle.Message, len(msgs))
	for i, m := range msgs {
		out[i] = nacelle.Message{
			Role:  m.Role,
			Parts: append([]nacelle.Part(nil), m.Parts...),
		}
	}
	return out
}
