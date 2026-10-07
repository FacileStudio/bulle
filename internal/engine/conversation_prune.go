package engine

import (
	"github.com/FacileStudio/nacelle"
)

func toolResultIDs(msg nacelle.Message) map[string]bool {
	if msg.Role != nacelle.RoleUser {
		return nil
	}
	ids := make(map[string]bool)
	for _, part := range msg.Parts {
		if tr, ok := part.(nacelle.ToolResult); ok {
			ids[tr.ID] = true
		}
	}
	return ids
}

func filterAssistantParts(parts []nacelle.Part, results map[string]bool) []nacelle.Part {
	kept := make([]nacelle.Part, 0, len(parts))
	for _, part := range parts {
		tc, ok := part.(nacelle.ToolCall)
		if !ok {
			kept = append(kept, part)
			continue
		}
		if results[tc.ID] {
			kept = append(kept, tc)
		}
	}
	return kept
}

func pruneOrphanedCalls(msgs []nacelle.Message) []nacelle.Message {
	out := make([]nacelle.Message, 0, len(msgs))
	for i, msg := range msgs {
		if msg.Role != nacelle.RoleAssistant {
			out = append(out, msg)
			continue
		}
		var results map[string]bool
		if i+1 < len(msgs) {
			results = toolResultIDs(msgs[i+1])
		}
		kept := filterAssistantParts(msg.Parts, results)
		if len(kept) > 0 {
			out = append(out, nacelle.Message{Role: nacelle.RoleAssistant, Parts: kept})
		}
	}
	return out
}

func alternate(msgs []nacelle.Message) []nacelle.Message {
	out := make([]nacelle.Message, 0, len(msgs))
	for _, msg := range msgs {
		if len(msg.Parts) == 0 {
			continue
		}
		if len(out) > 0 && out[len(out)-1].Role == msg.Role {
			last := &out[len(out)-1]
			last.Parts = append(last.Parts, msg.Parts...)
			continue
		}
		out = append(out, msg)
	}
	return out
}

// Replace updates the committed conversation messages.
func (c *Conversation) Replace(messages []nacelle.Message) {
	c.CloseResults()
	c.CloseTurn()
	c.messages = alternate(pruneOrphanedCalls(messages))
}
