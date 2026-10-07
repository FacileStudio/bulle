package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestSessionSubmitNilAgent(t *testing.T) {
	s := NewSession(nil, nil)
	_, err := s.Submit(context.Background(), "hello")
	if err == nil {
		t.Fatalf("expected error with nil agent")
	}
}

func TestSessionSubmitEmptyPromptEmptyConversation(t *testing.T) {
	b := &fakeBackend{}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	_, err := s.Submit(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error with empty prompt on empty conversation")
	}
}

func TestSessionSubmitTextStream(t *testing.T) {
	b := &fakeBackend{
		events: []nacelle.Event{
			{Kind: nacelle.KindText, Text: "hello world"},
			{Kind: nacelle.KindDone, Stop: nacelle.StopEnd},
		},
	}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	ch, err := s.Submit(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := collectEvents(ch)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if s.Conversation().Len() != 2 {
		t.Fatalf("expected 2 conversation messages, got %d", s.Conversation().Len())
	}
}

func TestSessionSubmitThinking(t *testing.T) {
	b := &fakeBackend{
		events: []nacelle.Event{
			{Kind: nacelle.KindThinking, Text: "deep thought"},
			{Kind: nacelle.KindDone, Stop: nacelle.StopEnd},
		},
	}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	ch, _ := s.Submit(context.Background(), "ponder")
	events := collectEvents(ch)
	if len(events) < 1 || events[0].Kind != EventThinking {
		t.Fatalf("expected thinking event")
	}
}

func TestSessionSubmitToolLoop(t *testing.T) {
	b := &fakeBackend{
		events: []nacelle.Event{
			{Kind: nacelle.KindText, Text: "calling tool"},
			{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "c1", Name: "bash", Input: `{"cmd":"ls"}`}},
			{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{ID: "c1", Name: "bash", Result: "file.txt"}},
			{Kind: nacelle.KindText, Text: "found file"},
			{Kind: nacelle.KindDone, Stop: nacelle.StopEnd},
		},
	}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	ch, _ := s.Submit(context.Background(), "list files")
	events := collectEvents(ch)
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}
	if s.Conversation().Len() != 4 {
		t.Fatalf("expected 4 messages in conversation, got %d", s.Conversation().Len())
	}
}

func TestSessionSubmitError(t *testing.T) {
	expectedErr := errors.New("backend failure")
	b := &fakeBackend{err: expectedErr}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	ch, _ := s.Submit(context.Background(), "fail")
	events := collectEvents(ch)
	if len(events) != 1 || events[0].Kind != EventError {
		t.Fatalf("expected 1 error event, got %v", events)
	}
}

func TestSessionSubmitMultiTurn(t *testing.T) {
	b := &fakeBackend{
		events: []nacelle.Event{
			{Kind: nacelle.KindText, Text: "reply"},
			{Kind: nacelle.KindDone, Stop: nacelle.StopEnd},
		},
	}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	ch1, _ := s.Submit(context.Background(), "turn 1")
	collectEvents(ch1)
	ch2, _ := s.Submit(context.Background(), "turn 2")
	collectEvents(ch2)
	if s.Conversation().Len() != 4 {
		t.Fatalf("expected 4 messages across 2 turns, got %d", s.Conversation().Len())
	}
}

func TestSessionSubmitContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := &fakeBackend{
		events: []nacelle.Event{{Kind: nacelle.KindText, Text: "stream"}},
	}
	agent := newTestAgent(t, b)
	s := NewSession(agent, nil)
	ch, _ := s.Submit(ctx, "cancel")
	collectEvents(ch)
}
