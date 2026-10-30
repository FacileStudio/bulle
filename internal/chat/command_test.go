package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		in, name, args string
		ok             bool
	}{
		{"/stop", "stop", "", true},
		{"  /STOP  ", "stop", "", true},
		{"/new", "new", "", true},
		{"/snooze", "snooze", "", true},
		{"/snooze until monday", "snooze", "until monday", true},
		{"/quiet 3", "quiet", "3", true},
		{"/forget my old address", "forget", "my old address", true},
		{"/status", "status", "", true},
		{"/stop\nnow", "stop", "now", true},
		{"/stopper", "", "", false},
		{"/unknown x", "", "", false},
		{"hello /stop", "", "", false},
		{"/", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		name, args, ok := parseCommand(tc.in)
		if name != tc.name || args != tc.args || ok != tc.ok {
			t.Errorf("parseCommand(%q) = %q, %q, %v", tc.in, name, args, ok)
		}
	}
}

type liveAdapter struct {
	fakeAdapter
	started chan struct{}
}

func (l *liveAdapter) Receive(_ context.Context, handle func(Message)) error {
	handle(l.in[0])
	<-l.started
	handle(l.in[1])
	return nil
}

func TestStopCancelsRunningTurn(t *testing.T) {
	a := &liveAdapter{started: make(chan struct{})}
	a.in = []Message{msg("!r:x", "@alice:x", "long job"), msg("!r:x", "@alice:x", "/stop")}
	respond := func(ctx context.Context, _ Message) (string, error) {
		close(a.started)
		<-ctx.Done()
		return "", ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		done <- RunWith(context.Background(), a, respond, Router{Allow: []string{"@alice:x"}}, &Commands{})
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("/stop did not cancel the running turn")
	}
	got := map[string]bool{}
	for _, r := range a.replies() {
		got[r.Text] = true
	}
	if !got["stopped."] || len(got) != 1 {
		t.Fatalf("replies = %v, want a single stopped.", got)
	}
}

func TestCommandRunsWithoutModel(t *testing.T) {
	a := &fakeAdapter{in: []Message{msg("!r:x", "@alice:x", "/quiet 2"), msg("!r:x", "@alice:x", "/unknown")}}
	cmds := &Commands{Quiet: func(_ context.Context, _ Message, args string) string { return "quiet " + args }}
	if err := RunWith(context.Background(), a, echo, Router{Allow: []string{"@alice:x"}}, cmds); err != nil {
		t.Fatal(err)
	}
	r := a.replies()
	if len(r) != 2 || r[0].Text != "quiet 2" || r[1].Text != "answer to /unknown" {
		t.Fatalf("replies = %+v", r)
	}
}

func TestReactionReachesHandler(t *testing.T) {
	router := NewReactionRouter()
	router.Register("$nudge", []string{"k1", "k2"})
	var gotKeys []string
	var gotEmoji string
	cmds := &Commands{Reactions: router, OnReaction: func(_ context.Context, m Message, keys []string) bool {
		gotKeys, gotEmoji = keys, m.Reaction
		return true
	}}
	hit := Message{Identity: Identity{Adapter: "fake", Room: "!r:x", Sender: "@alice:x"}, Kind: KindReaction, Reaction: "💤", Target: "$nudge"}
	miss := hit
	miss.Target = "$other"
	a := &fakeAdapter{in: []Message{miss, hit}}
	if err := RunWith(context.Background(), a, echo, Router{Allow: []string{"@alice:x"}}, cmds); err != nil {
		t.Fatal(err)
	}
	if strings.Join(gotKeys, ",") != "k1,k2" || gotEmoji != "💤" || len(a.replies()) != 0 {
		t.Fatalf("keys=%v emoji=%q replies=%v", gotKeys, gotEmoji, a.replies())
	}
}

func TestReactionRouterIsBounded(t *testing.T) {
	r := NewReactionRouter()
	for i := 0; i < reactionRouterSize+10; i++ {
		r.Register(strings.Repeat("e", i+1), []string{"k"})
	}
	if r.Lookup("e") != nil || r.Lookup(strings.Repeat("e", reactionRouterSize+10)) == nil {
		t.Fatal("oldest event should be evicted and newest kept")
	}
}

func TestAudioExtraction(t *testing.T) {
	file := &event.EncryptedFileInfo{URL: "mxc://x/enc"}
	cases := []struct {
		name    string
		content event.MessageEventContent
		ok      bool
		url     string
	}{
		{"audio", event.MessageEventContent{MsgType: event.MsgAudio, URL: "mxc://x/a", Info: &event.FileInfo{MimeType: "audio/ogg", Size: 42}}, true, "mxc://x/a"},
		{"file audio", event.MessageEventContent{MsgType: event.MsgFile, URL: "mxc://x/f", Info: &event.FileInfo{MimeType: "audio/mpeg"}}, true, "mxc://x/f"},
		{"encrypted", event.MessageEventContent{MsgType: event.MsgAudio, File: file}, true, "mxc://x/enc"},
		{"file pdf", event.MessageEventContent{MsgType: event.MsgFile, URL: "mxc://x/p", Info: &event.FileInfo{MimeType: "application/pdf"}}, false, ""},
		{"image", event.MessageEventContent{MsgType: event.MsgImage, URL: "mxc://x/i"}, false, ""},
	}
	for _, tc := range cases {
		evt := &event.Event{Sender: "@alice:x", RoomID: "!r:x", ID: "$e", Type: event.EventMessage,
			Content: event.Content{Parsed: &tc.content}}
		m, ok := messageFrom(evt, "@bot:x")
		if ok != tc.ok {
			t.Fatalf("%s: ok = %v", tc.name, ok)
		}
		if ok && (!m.Is(KindAudio) || m.Media.URL != tc.url) {
			t.Fatalf("%s: %+v", tc.name, m)
		}
	}
	m, _ := messageFrom(&event.Event{Sender: "@alice:x", Type: event.EventMessage,
		Content: event.Content{Parsed: &event.MessageEventContent{MsgType: event.MsgAudio, File: file}}}, "@bot:x")
	var info map[string]any
	if json.Unmarshal([]byte(m.Media.Key), &info) != nil || info["url"] != "mxc://x/enc" {
		t.Fatalf("key = %q", m.Media.Key)
	}
}

func TestReactionExtraction(t *testing.T) {
	content := &event.ReactionEventContent{RelatesTo: event.RelatesTo{Type: event.RelAnnotation, EventID: "$t", Key: "👎"}}
	evt := &event.Event{Sender: "@alice:x", RoomID: "!r:x", ID: "$r", Type: event.EventReaction, Content: event.Content{Parsed: content}}
	m, ok := reactionFrom(evt, "@bot:x")
	if !ok || m.Reaction != "👎" || m.Target != "$t" || !m.Is(KindReaction) {
		t.Fatalf("%+v %v", m, ok)
	}
	if _, ok := reactionFrom(evt, id.UserID("@alice:x")); ok {
		t.Fatal("own reaction must be ignored")
	}
}

func TestNoticeContent(t *testing.T) {
	to := Identity{Adapter: "matrix", Room: "!r:x", Sender: "@alice:x"}
	if got := contentAs(to, "digest", event.MsgNotice).MsgType; got != event.MsgNotice {
		t.Fatalf("msgtype = %s", got)
	}
	if got := contentAs(to, "reply", event.MsgText).MsgType; got != event.MsgText {
		t.Fatalf("msgtype = %s", got)
	}
}
