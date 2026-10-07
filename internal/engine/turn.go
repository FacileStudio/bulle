package engine

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/FacileStudio/nacelle"
)

// Submit submits a user prompt and begins streaming events for the turn.
func (s *Session) Submit(ctx context.Context, prompt string) (<-chan Event, error) {
	if s.agent == nil {
		return nil, errors.New("engine: agent is required")
	}
	if prompt != "" {
		s.conv.AppendUserText(prompt)
	}
	if s.conv.Len() == 0 {
		return nil, errors.New("engine: prompt is required for empty conversation")
	}
	events := make(chan Event, 64)
	go s.runStream(ctx, events)
	return events, nil
}

func (s *Session) runStream(ctx context.Context, events chan<- Event) {
	defer close(events)
	for event, err := range s.agent.Stream(ctx, s.conv.Messages()) {
		if err != nil {
			emit(ctx, events, NewError(err))
			return
		}
		if !s.handleStreamEvent(ctx, event, events) {
			return
		}
	}
}

func (s *Session) handleStreamEvent(ctx context.Context, event nacelle.Event, events chan<- Event) bool {
	switch event.Kind {
	case nacelle.KindText:
		s.conv.AppendAssistantText(event.Text)
		return emit(ctx, events, NewTextDelta(event.Text))
	case nacelle.KindThinking:
		return emit(ctx, events, NewThinking(event.Text))
	case nacelle.KindToolCall:
		if event.Tool != nil {
			s.conv.RecordToolCall(toolCallFromEvent(event.Tool))
			return emit(ctx, events, NewToolCall(event.Tool))
		}
	case nacelle.KindToolResult:
		if event.Tool != nil {
			if event.Tool.Discarded {
				s.conv.DiscardToolCall(event.Tool.ID)
				return true
			}
			s.conv.RecordToolResult(toolResultFromEvent(event.Tool))
			return emit(ctx, events, NewToolResult(event.Tool))
		}
	case nacelle.KindDone:
		s.conv.CloseTurn(event.Stop)
		s.conv.CloseResults()
		return emit(ctx, events, NewTurnDone(event.Usage, event.Stop))
	}
	return true
}

func emit(ctx context.Context, events chan<- Event, ev Event) bool {
	select {
	case events <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

func toolCallFromEvent(tool *nacelle.ToolEvent) nacelle.ToolCall {
	return nacelle.ToolCall{
		ID:       tool.ID,
		Name:     tool.Name,
		Input:    json.RawMessage(tool.Input),
		Finished: true,
	}
}

func toolResultFromEvent(tool *nacelle.ToolEvent) nacelle.ToolResult {
	return nacelle.ToolResult{
		ID:     tool.ID,
		Name:   tool.Name,
		Result: tool.Result,
		Failed: tool.Err != nil,
	}
}
