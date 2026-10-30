package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/engine"
)

func testRoom(path string, now time.Time, summary string) *chatRoom {
	day, msgs := loadRoom(path)
	return &chatRoom{
		session:   engine.NewSession(nil, engine.NewConversation(msgs...)),
		path:      path,
		day:       day,
		loc:       time.UTC,
		now:       func() time.Time { return now },
		summarize: func(context.Context, []nacelle.Message) string { return summary },
	}
}

func joined(r *chatRoom) string {
	var b strings.Builder
	for _, m := range r.session.Conversation().Messages() {
		b.WriteString(string(m.Role) + ":" + roomText(m) + "\n")
	}
	return b.String()
}

func TestRoomPersistsAcrossRebuild(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := roomStorePath("matrix:!room")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	first := testRoom(path, now, "")
	first.rollDay(context.Background())
	conv := first.session.Conversation()
	conv.AppendUserText("hello")
	conv.AppendAssistantText("hi there")
	first.persist()
	first.record("nudge: task overdue")

	second := testRoom(path, now, "")
	got := joined(second)
	for _, want := range []string{"user:hello", "assistant:hi there", "nudge: task overdue"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Index(got, "hi there") > strings.Index(got, "nudge") {
		t.Fatalf("nudge out of order: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode: %v %v", info, err)
	}
	dir, _ := os.Stat(filepath.Dir(path))
	if dir.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode: %v", dir.Mode())
	}
}

func TestRoomDayBoundaryCarriesSummary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, _ := roomStorePath("matrix:!day")
	evening := time.Date(2026, 10, 30, 23, 0, 0, 0, time.UTC)
	room := testRoom(path, evening, "talked about the landing page")
	room.rollDay(context.Background())
	room.session.Conversation().AppendUserText("old question")
	room.session.Conversation().AppendAssistantText("old answer")

	room.now = func() time.Time { return time.Date(2026, 10, 31, 3, 59, 0, 0, time.UTC) }
	room.rollDay(context.Background())
	if !strings.Contains(joined(room), "old question") {
		t.Fatal("conversation reset before 04:00")
	}

	room.now = func() time.Time { return time.Date(2026, 10, 31, 4, 1, 0, 0, time.UTC) }
	room.rollDay(context.Background())
	got := joined(room)
	if strings.Contains(got, "old question") || !strings.Contains(got, "talked about the landing page") {
		t.Fatalf("unexpected conversation: %q", got)
	}
	if room.session.Conversation().Len() != 1 {
		t.Fatalf("want one carried message, got %q", got)
	}
}

func TestRoomResetStartsFresh(t *testing.T) {
	room := testRoom("", time.Now(), "")
	room.session.Conversation().AppendUserText("x")
	room.reset(context.Background())
	if room.session.Conversation().Len() != 0 {
		t.Fatal("reset kept messages")
	}
}

func TestEmptyReplyNotice(t *testing.T) {
	cases := map[string]string{
		"Salut, ça va ?":       "Je n'ai rien à ajouter.",
		"peux-tu regarder ça":  "Je n'ai rien à ajouter.",
		"what is the weather?": "Nothing to add.",
	}
	for in, want := range cases {
		if got := emptyReplyNotice(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
