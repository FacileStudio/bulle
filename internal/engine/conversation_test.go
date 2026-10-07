package engine

import (
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestConversationEmpty(t *testing.T) {
	c := NewConversation()
	if c.Len() != 0 {
		t.Fatalf("expected len 0, got %d", c.Len())
	}
	if len(c.Messages()) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(c.Messages()))
	}
}

func TestConversationAppendUserText(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("hello")
	if c.Len() != 1 {
		t.Fatalf("expected len 1, got %d", c.Len())
	}
	msgs := c.Messages()
	if msgs[0].Role != nacelle.RoleUser {
		t.Fatalf("expected role user, got %s", msgs[0].Role)
	}
}

func TestConversationRoleAlternationUser(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("first")
	c.AppendUserText("second")
	if c.Len() != 1 {
		t.Fatalf("expected len 1 after user fold, got %d", c.Len())
	}
	if len(c.Messages()[0].Parts) != 2 {
		t.Fatalf("expected 2 parts folded, got %d", len(c.Messages()[0].Parts))
	}
}

func TestConversationRoleAlternationAssistant(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("prompt")
	c.AppendAssistantText("answer 1")
	c.CloseTurn()
	c.AppendAssistantText("answer 2")
	c.CloseTurn()
	if c.Len() != 2 {
		t.Fatalf("expected 2 messages after assistant fold, got %d", c.Len())
	}
}

func TestConversationToolPairing(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("read file")
	c.AppendAssistantText("reading")
	c.RecordToolCall(nacelle.ToolCall{ID: "c1", Name: "read"})
	c.RecordToolResult(nacelle.ToolResult{ID: "c1", Name: "read", Result: "data"})
	c.AppendAssistantText("done")
	c.CloseTurn(nacelle.StopEnd)
	if c.Len() != 4 {
		t.Fatalf("expected 4 messages, got %d", c.Len())
	}
}

func TestConversationOrphanedToolCallDropped(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("question")
	c.AppendAssistantText("let me look")
	c.RecordToolCall(nacelle.ToolCall{ID: "orphan", Name: "read"})
	c.CloseTurn()
	msgs := c.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	for _, p := range msgs[1].Parts {
		if _, ok := p.(nacelle.ToolCall); ok {
			t.Fatalf("expected orphaned tool call to be dropped")
		}
	}
}

func TestConversationDiscardedToolCall(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("question")
	c.RecordToolCall(nacelle.ToolCall{ID: "disc", Name: "read"})
	c.DiscardToolCall("disc")
	c.CloseTurn()
	msgs := c.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message since empty assistant was omitted, got %d", len(msgs))
	}
}

func TestConversationSnapshotIsolation(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("original")
	snap := c.Snapshot()
	snap[0].Parts = append(snap[0].Parts, nacelle.Text{Text: "mutation"})
	if len(c.Messages()[0].Parts) != 1 {
		t.Fatalf("expected snapshot to not affect conversation")
	}
}

func TestConversationReplace(t *testing.T) {
	c := NewConversation()
	c.AppendUserText("original")
	if c.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", c.Len())
	}
	c.Replace([]nacelle.Message{nacelle.UserText("replaced")})
	msgs := c.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	text, ok := msgs[0].Parts[0].(nacelle.Text)
	if !ok || text.Text != "replaced" {
		t.Fatalf("expected replaced text, got %v", msgs[0].Parts[0])
	}
}
