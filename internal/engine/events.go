package engine

import (
	"github.com/FacileStudio/nacelle"
)

// EventKind identifies what an Event carries.
type EventKind string

const (
	EventTextDelta  EventKind = "text_delta"
	EventToolCall   EventKind = "tool_call"
	EventToolResult EventKind = "tool_result"
	EventThinking   EventKind = "thinking"
	EventTurnDone   EventKind = "turn_done"
	EventError      EventKind = "error"

	TextDelta  = EventTextDelta
	ToolCall   = EventToolCall
	ToolResult = EventToolResult
	Thinking   = EventThinking
	TurnDone   = EventTurnDone
	Error      = EventError
)

// Event is a typed notification emitted during agent execution.
type Event struct {
	Kind  EventKind
	Text  string
	Tool  *nacelle.ToolEvent
	Usage nacelle.Usage
	Stop  nacelle.Stop
	Err   error
}

// NewTextDelta creates a text delta event.
func NewTextDelta(text string) Event {
	return Event{Kind: EventTextDelta, Text: text}
}

// NewThinking creates a thinking delta event.
func NewThinking(text string) Event {
	return Event{Kind: EventThinking, Text: text}
}

// NewToolCall creates a tool call event.
func NewToolCall(tool *nacelle.ToolEvent) Event {
	return Event{Kind: EventToolCall, Tool: tool}
}

// NewToolResult creates a tool result event.
func NewToolResult(tool *nacelle.ToolEvent) Event {
	return Event{Kind: EventToolResult, Tool: tool}
}

// NewTurnDone creates a turn completion event.
func NewTurnDone(usage nacelle.Usage, stop nacelle.Stop) Event {
	return Event{Kind: EventTurnDone, Usage: usage, Stop: stop}
}

// NewError creates an error event.
func NewError(err error) Event {
	return Event{Kind: EventError, Err: err}
}
