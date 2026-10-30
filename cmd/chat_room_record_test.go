package cmd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/engine"
)

func newRecordRoom() *chatRoom {
	return &chatRoom{session: engine.NewSession(nil, engine.NewConversation())}
}

func TestRecordNeverBlocksOnBusyRoom(t *testing.T) {
	r := newRecordRoom()
	r.mu.Lock()
	done := make(chan struct{})
	go func() {
		r.record("nudge")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("record blocked on the room lock")
	}
	if r.session.Conversation().Len() != 0 {
		t.Fatal("record wrote while the room was busy")
	}
	r.flushPending()
	r.mu.Unlock()
	msgs := r.session.Conversation().Messages()
	if len(msgs) != 2 || msgs[0].Role != nacelle.RoleUser || msgs[1].Role != nacelle.RoleAssistant {
		t.Fatalf("history = %+v", msgs)
	}
}

func TestRecordIdleRoomStartsWithUserMessage(t *testing.T) {
	r := newRecordRoom()
	r.record("first")
	r.record("second")
	msgs := r.session.Conversation().Messages()
	if len(msgs) != 2 || msgs[0].Role != nacelle.RoleUser || roomText(msgs[0]) != neutralUser {
		t.Fatalf("history = %+v", msgs)
	}
}

func TestLoadRoomRestoresUserFirst(t *testing.T) {
	msgs := userFirst([]nacelle.Message{{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "hi"}}}})
	if len(msgs) != 2 || msgs[0].Role != nacelle.RoleUser {
		t.Fatalf("history = %+v", msgs)
	}
	same := userFirst(msgs)
	if len(same) != 2 {
		t.Fatalf("second pass changed history: %+v", same)
	}
}

func TestAnswerHonoursCanceledContextWhileBusy(t *testing.T) {
	r := newRecordRoom()
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.answer(ctx, "hi"); err == nil {
		t.Fatal("answer must give up when the context ends")
	}
}

func TestRecordedTextPrefersTemplate(t *testing.T) {
	got := recordedText(chat.IPCRequest{Text: "model prose", RecordText: "template"})
	if got != "template" || recordedText(chat.IPCRequest{Text: "plain"}) != "plain" {
		t.Fatalf("recordedText = %q", got)
	}
}

func TestQuietRejectsNaNAndInf(t *testing.T) {
	s := testServices(t)
	for _, arg := range []string{"NaN", "Inf", "+Inf", "-Inf", "0", "73"} {
		if got := s.cmdQuiet(context.Background(), chat.Message{}, arg); !strings.HasPrefix(got, "Usage") {
			t.Fatalf("%s accepted: %q", arg, got)
		}
	}
}

func TestSnoozeLabelSaysTodayBeforeNine(t *testing.T) {
	loc := time.UTC
	early := time.Date(2026, 10, 30, 7, 0, 0, 0, loc)
	late := time.Date(2026, 10, 30, 15, 0, 0, 0, loc)
	nine := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, loc) }
	if got := snoozeLabel(nine(early), early, loc); got != "aujourd'hui 9h" {
		t.Fatalf("early = %q", got)
	}
	if got := snoozeLabel(nine(late.AddDate(0, 0, 1)), late, loc); got != "demain 9h" {
		t.Fatalf("late = %q", got)
	}
}
