package cmd

import (
	"context"
	"testing"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/bulle/internal/settings"
)

func TestRoomRegistryKeyIsolation(t *testing.T) {
	reg := newRoomRegistry(settings.Defaults(""))
	room1 := &chatRoom{session: engine.NewSession(nil, nil)}
	room2 := &chatRoom{session: engine.NewSession(nil, nil)}
	reg.rooms["room1"] = room1
	reg.rooms["room2"] = room2

	got1, err := reg.get("room1")
	if err != nil || got1 != room1 {
		t.Fatalf("expected room1, got %v (err: %v)", got1, err)
	}
	got2, err := reg.get("room2")
	if err != nil || got2 != room2 {
		t.Fatalf("expected room2, got %v (err: %v)", got2, err)
	}
	if got1 == got2 {
		t.Fatalf("expected different rooms for different keys")
	}
}

func TestRoomRegistryCloseCleansUp(t *testing.T) {
	reg := newRoomRegistry(settings.Defaults(""))
	cleaned := false
	reg.rooms["test"] = &chatRoom{
		cleanup: func() { cleaned = true },
	}
	if err := reg.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !cleaned {
		t.Fatalf("expected cleanup to be called on Close")
	}
}

func TestRoomCompactIfNeeded(t *testing.T) {
	conv := engine.NewConversation()
	conv.AppendUserText("first message with some tokens to reach threshold")
	conv.AppendAssistantText("second message with even more tokens to reach compaction threshold")
	conv.CloseTurn()

	policy := compaction.Policy{
		Ceiling: 1,
	}
	compactor := compaction.NewEngine(policy, nil, nil)
	room := &chatRoom{
		session:   engine.NewSession(nil, conv),
		compactor: compactor,
	}
	room.compactIfNeeded(context.Background())
	if conv.Len() == 0 {
		t.Fatalf("expected messages to remain after compaction")
	}
}

func TestRoomDrainText(t *testing.T) {
	room := &chatRoom{}
	events := make(chan engine.Event, 3)
	events <- engine.NewTextDelta("hello ")
	events <- engine.NewTextDelta("world")
	events <- engine.NewTurnDone(nacelle.Usage{}, nacelle.Stop("end_turn"))
	close(events)

	got, err := room.drain(context.Background(), events)
	if err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}
