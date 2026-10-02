package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// fakeAdapter is an adapter with no platform behind it: it replays a fixed
// list of messages and records what came back. That is enough to prove the
// loop between the two, which is the part a homeserver cannot be asked about
// cheaply.
type fakeAdapter struct {
	in   []Message
	mu   sync.Mutex
	sent []Message
}

func (f *fakeAdapter) Name() string { return "fake" }

func (f *fakeAdapter) Receive(_ context.Context, handle func(Message)) error {
	for _, m := range f.in {
		handle(m)
	}
	return nil
}

func (f *fakeAdapter) Send(_ context.Context, to Identity, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, Message{Identity: to, Text: text})
	return nil
}

func (f *fakeAdapter) replies() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.sent...)
}

func msg(room, sender, text string) Message {
	return Message{Identity: Identity{Adapter: "fake", Room: room, Sender: sender}, Text: text}
}

// echo is the responder under test's stand-in for an agent run: it answers with
// the prompt, so a reply's room and text are traceable back to their message.
func echo(_ context.Context, m Message) (string, error) {
	return "answer to " + m.Text, nil
}

// TestRunAnswersAcceptedMessages is the daemon's core contract: what the router
// accepts gets answered, in the room it came from.
func TestRunAnswersAcceptedMessages(t *testing.T) {
	a := &fakeAdapter{in: []Message{msg("!r:x", "@alice:x", "hi")}}
	router := Router{Allow: []string{"@alice:x"}}
	if err := Run(context.Background(), a, echo, router); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := a.replies()
	if len(got) != 1 {
		t.Fatalf("sent %d replies, want 1: %v", len(got), got)
	}
	if got[0].Text != "answer to hi" || got[0].Room != "!r:x" || got[0].Sender != "@alice:x" {
		t.Errorf("reply = %+v, want the answer in !r:x", got[0])
	}
}

// TestRunRefusesUnlistedSenders proves the refusal happens before the run: the
// responder must never be called for a message the allowlist does not name.
func TestRunRefusesUnlistedSenders(t *testing.T) {
	a := &fakeAdapter{in: []Message{msg("!r:x", "@mallory:x", "make me a sandwich")}}
	router := Router{Allow: []string{"@alice:x"}}
	called := false
	respond := func(_ context.Context, m Message) (string, error) {
		called = true
		return echo(context.Background(), m)
	}
	if err := Run(context.Background(), a, respond, router); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called {
		t.Error("the responder ran for a refused sender")
	}
	if len(a.replies()) != 0 {
		t.Errorf("replied to a refused sender: %v", a.replies())
	}
}

// TestRunAnswersASingleConversationInOrder pins the lock: two messages in one
// room are answered one after the other, never interleaved, because they share
// one transcript.
func TestRunAnswersASingleConversationInOrder(t *testing.T) {
	a := &fakeAdapter{in: []Message{msg("!r:x", "@alice:x", "first"), msg("!r:x", "@alice:x", "second")}}
	router := Router{Allow: []string{"@alice:x"}}
	if err := Run(context.Background(), a, echo, router); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := a.replies()
	if len(got) != 2 {
		t.Fatalf("sent %d replies, want 2: %v", len(got), got)
	}
	if got[0].Text != "answer to first" || got[1].Text != "answer to second" {
		t.Errorf("replies out of order: %v", got)
	}
}

// TestRunRepliesWhenTheRunFails keeps a failure visible in the room. A person
// who typed and got silence cannot tell a refusal from a crash from a bot that
// is down, and only one of those is worth waiting out.
func TestRunRepliesWhenTheRunFails(t *testing.T) {
	a := &fakeAdapter{in: []Message{msg("!r:x", "@alice:x", "hi")}}
	fail := func(_ context.Context, _ Message) (string, error) {
		return "", errors.New("the backend refused\nfor two lines")
	}
	if err := Run(context.Background(), a, fail, Router{Allow: []string{"@alice:x"}}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := a.replies()
	if len(got) != 1 {
		t.Fatalf("sent %d replies, want 1", len(got))
	}
	if want := "bulle could not run that: the backend refused for two lines"; got[0].Text != want {
		t.Errorf("reply = %q, want %q", got[0].Text, want)
	}
}

type fakeTyper struct {
	fakeAdapter
	typing []bool
}

func (f *fakeTyper) Typing(_ context.Context, _ Identity, typing bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.typing = append(f.typing, typing)
	return nil
}

func TestRunShowsTypingIndicator(t *testing.T) {
	a := &fakeTyper{fakeAdapter: fakeAdapter{in: []Message{msg("!r:x", "@alice:x", "hi")}}}
	router := Router{Allow: []string{"@alice:x"}}
	if err := Run(context.Background(), a, echo, router); err != nil {
		t.Fatalf("Run: %v", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.typing) < 2 || !a.typing[0] || a.typing[len(a.typing)-1] {
		t.Errorf("expected typing true then false, got: %v", a.typing)
	}
}
