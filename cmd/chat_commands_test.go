package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/memory"
	"github.com/FacileStudio/bulle/internal/settings"
)

func testServices(t *testing.T) *chatServices {
	t.Helper()
	dir := t.TempDir()
	ledger, err := assistant.NewLedger(filepath.Join(dir, "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	reactions := chat.NewReactionRouter()
	rooms := newRoomRegistry(settings.Config{})
	t.Cleanup(func() {
		if err := rooms.Close(); err != nil {
			t.Error(err)
		}
	})
	rooms.nudges = newNudgeLog(reactions)
	return &chatServices{
		rooms:     rooms,
		ledger:    ledger,
		memory:    memory.New(filepath.Join(dir, "memory")),
		approvals: newApprovals(nil, reactions),
		stateDir:  filepath.Join(dir, "state"),
	}
}

func TestQuietWritesUntilAndRejectsBadHours(t *testing.T) {
	s := testServices(t)
	if got := s.cmdQuiet(context.Background(), chat.Message{}, "abc"); !strings.HasPrefix(got, "Usage") {
		t.Fatalf("bad hours reply = %q", got)
	}
	if got := s.cmdQuiet(context.Background(), chat.Message{}, "2"); !strings.HasPrefix(got, "Silence jusqu'à") {
		t.Fatalf("reply = %q", got)
	}
	if !assistant.InQuiet(time.Now().Add(time.Hour), s.assistant, s.stateDir) {
		t.Fatal("quiet.json must silence the next hour")
	}
}

func TestSnoozeUsesTheLastNudge(t *testing.T) {
	s := testServices(t)
	m := chat.Message{Identity: chat.Identity{Room: "!r:x"}}
	if got := s.cmdSnooze(context.Background(), m, ""); got != "Rien à reporter." {
		t.Fatalf("empty reply = %q", got)
	}
	s.rooms.nudges.note("!r:x", "$e", []string{"task:7"})
	if got := s.cmdSnooze(context.Background(), m, ""); !strings.Contains(got, "1 élément") {
		t.Fatalf("reply = %q", got)
	}
	item := assistant.Item{Source: assistant.SourceTask, ID: "7", Status: "in-progress"}
	if s.ledger.ShouldSurface(item, time.Now()) {
		t.Fatal("a snoozed item must not surface")
	}
}

func TestReactionsSnoozeAndMarkNoise(t *testing.T) {
	s := testServices(t)
	ctx := context.Background()
	if !s.onReaction(ctx, chat.Message{Reaction: "👎"}, []string{"mail:abc"}) {
		t.Fatal("thumbs down must be handled")
	}
	if s.ledger.ShouldSurface(assistant.Item{Source: assistant.SourceMail, ID: "abc"}, time.Now()) {
		t.Fatal("a noise item must not surface")
	}
	if s.onReaction(ctx, chat.Message{Reaction: "🎉"}, []string{"mail:abc"}) {
		t.Fatal("an unknown emoji must be left alone")
	}
}

func TestForgetReportsLinesRemoved(t *testing.T) {
	s := testServices(t)
	if _, err := s.memory.Remember("Yann likes tea", time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := s.cmdForget(context.Background(), chat.Message{}, "likes tea"); !strings.HasPrefix(got, "Oublié : 1 ligne(s). Sauvegardes (backups/) : ") {
		t.Fatalf("reply = %q", got)
	}
}

func TestCarriedMessageHoldsMemoryAndSummary(t *testing.T) {
	mem := memory.New(t.TempDir())
	if err := mem.Append(memory.UserFile, "- Yann lives in Lyon"); err != nil {
		t.Fatal(err)
	}
	msgs := carriedMessage("worked on bulle", mem, time.Now())
	if len(msgs) != 1 {
		t.Fatalf("messages = %d", len(msgs))
	}
	text := msgs[0].Parts[0].(nacelle.Text).Text
	if !strings.Contains(text, "Lyon") || !strings.Contains(text, "worked on bulle") {
		t.Fatalf("text = %q", text)
	}
	if carriedMessage("", nil, time.Now()) != nil {
		t.Fatal("nothing to carry means no message")
	}
}
