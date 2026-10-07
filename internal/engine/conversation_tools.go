package engine

import (
	"github.com/FacileStudio/nacelle"
)

// RecordToolCall buffers a tool invocation requested by the assistant.
func (c *Conversation) RecordToolCall(call nacelle.ToolCall) {
	if len(c.answered) > 0 {
		c.CloseResults()
	}
	c.asked = append(c.asked, call)
}

// RecordToolResult buffers a tool execution result.
func (c *Conversation) RecordToolResult(result nacelle.ToolResult) {
	if len(c.asked) > 0 || c.pendingText.Len() > 0 {
		c.CloseTurn()
	}
	c.answered = append(c.answered, result)
}

// DiscardToolCall removes a pending tool call that was discarded before execution.
func (c *Conversation) DiscardToolCall(id string) {
	kept := c.asked[:0]
	for _, call := range c.asked {
		if call.ID != id {
			kept = append(kept, call)
		}
	}
	c.asked = kept
}

// CloseTurn commits the pending assistant turn to history.
func (c *Conversation) CloseTurn(stops ...nacelle.Stop) {
	var stop nacelle.Stop
	if len(stops) > 0 {
		stop = stops[0]
	}
	text := c.pendingText.String()
	c.pendingText.Reset()
	parts := make([]nacelle.Part, 0, len(c.asked)+2)
	if text != "" {
		parts = append(parts, nacelle.Text{Text: text})
	}
	for _, call := range c.asked {
		parts = append(parts, call)
	}
	c.asked = nil
	if stop != "" {
		parts = append(parts, nacelle.Finish{Stop: stop})
	}
	c.commit(nacelle.RoleAssistant, parts)
}

func (c *Conversation) commit(role nacelle.Role, parts []nacelle.Part) {
	if len(parts) == 0 {
		return
	}
	if len(c.messages) > 0 && c.messages[len(c.messages)-1].Role == role {
		last := &c.messages[len(c.messages)-1]
		last.Parts = append(last.Parts, parts...)
		return
	}
	c.messages = append(c.messages, nacelle.Message{Role: role, Parts: parts})
}

func (c *Conversation) answeredParts() []nacelle.Part {
	parts := make([]nacelle.Part, 0, len(c.answered))
	for _, res := range c.answered {
		parts = append(parts, res)
	}
	return parts
}
