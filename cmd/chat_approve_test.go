package cmd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

type cardAdapter struct {
	mu    sync.Mutex
	cards []string
	texts []string
	sent  chan string
}

func (c *cardAdapter) Name() string { return "matrix" }

func (c *cardAdapter) Receive(context.Context, func(chat.Message)) error { return nil }

func (c *cardAdapter) Send(_ context.Context, _ chat.Identity, text string) error {
	c.mu.Lock()
	c.texts = append(c.texts, text)
	c.mu.Unlock()
	return nil
}

func (c *cardAdapter) SendQuiet(_ context.Context, _ chat.Identity, text string) (string, error) {
	c.mu.Lock()
	c.texts = append(c.texts, text)
	c.mu.Unlock()
	return "", nil
}

func (c *cardAdapter) SendEvent(_ context.Context, _ chat.Identity, text string) (string, error) {
	c.mu.Lock()
	c.cards = append(c.cards, text)
	c.mu.Unlock()
	c.sent <- "$card"
	return "$card", nil
}

type askResult struct {
	ok  bool
	err error
}

func startAsk(t *testing.T, a *approvals, ad *cardAdapter) chan askResult {
	t.Helper()
	out := make(chan askResult, 1)
	go func() {
		ok, err := a.Ask(context.Background(), "matrix:!r:s", "`courrier send`")
		out <- askResult{ok, err}
	}()
	select {
	case <-ad.sent:
	case <-time.After(2 * time.Second):
		t.Fatal("card not posted")
	}
	waitBound(t, a)
	return out
}

func waitBound(t *testing.T, a *approvals) {
	t.Helper()
	for i := 0; i < 200; i++ {
		a.mu.Lock()
		cards := a.pending["matrix:!r:s"]
		bound := len(cards) > 0 && cards[0].event != ""
		a.mu.Unlock()
		if bound {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("card never bound")
}

func newTestApprovals() (*approvals, *cardAdapter, chan time.Time) {
	ad := &cardAdapter{sent: make(chan string, 4)}
	a := newApprovals(ad, chat.NewReactionRouter())
	clock := make(chan time.Time, 1)
	a.timer = func(time.Duration) (<-chan time.Time, func()) { return clock, func() {} }
	return a, ad, clock
}

func reply(text string) chat.Message {
	return chat.Message{Identity: chat.Identity{Adapter: "matrix", Room: "!r:s"}, Text: text}
}

func react(emoji, target string) chat.Message {
	m := reply("")
	m.Kind, m.Reaction, m.Target = chat.KindReaction, emoji, target
	return m
}

func result(t *testing.T, ch chan askResult) askResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("ask did not return")
	}
	return askResult{}
}

func TestApprovalReactionApproves(t *testing.T) {
	a, ad, _ := newTestApprovals()
	ch := startAsk(t, a, ad)
	if len(ad.cards) != 1 || ad.cards[0] == "" {
		t.Fatalf("cards = %v", ad.cards)
	}
	if !a.HandleReaction(context.Background(), react("✅", "$card"), nil) {
		t.Fatal("reaction not handled")
	}
	if r := result(t, ch); !r.ok || r.err != nil {
		t.Fatalf("result = %+v", r)
	}
}

func TestApprovalReactionDenies(t *testing.T) {
	a, ad, _ := newTestApprovals()
	ch := startAsk(t, a, ad)
	a.HandleReaction(context.Background(), react("❌", "$card"), nil)
	if r := result(t, ch); r.ok {
		t.Fatal("denied reaction approved")
	}
}

func TestApprovalUnknownReactionIgnored(t *testing.T) {
	a, ad, _ := newTestApprovals()
	ch := startAsk(t, a, ad)
	if a.HandleReaction(context.Background(), react("✅", "$other"), nil) {
		t.Fatal("unknown event handled")
	}
	if a.HandleReaction(context.Background(), react("👍", "$card"), nil) {
		t.Fatal("unrelated emoji handled")
	}
	a.Intercept(reply("non"))
	result(t, ch)
}

func TestApprovalTextReply(t *testing.T) {
	a, ad, _ := newTestApprovals()
	if a.Intercept(reply("oui")) {
		t.Fatal("intercepted with nothing pending")
	}
	ch := startAsk(t, a, ad)
	if a.Intercept(reply("peux-tu relire")) {
		t.Fatal("intercepted a non-verdict")
	}
	if !a.Intercept(reply("Oui")) {
		t.Fatal("verdict not intercepted")
	}
	if r := result(t, ch); !r.ok {
		t.Fatal("oui did not approve")
	}
	if a.Intercept(reply("oui")) {
		t.Fatal("intercepted after resolution")
	}
}

func TestApprovalTimeoutAndCancel(t *testing.T) {
	a, ad, clock := newTestApprovals()
	ch := startAsk(t, a, ad)
	clock <- time.Now()
	if r := result(t, ch); r.ok || r.err != nil {
		t.Fatalf("timeout result = %+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan askResult, 1)
	go func() {
		ok, err := a.Ask(ctx, "matrix:!r:s", "x")
		done <- askResult{ok, err}
	}()
	<-ad.sent
	cancel()
	if r := result(t, done); r.ok || r.err == nil {
		t.Fatalf("cancel result = %+v", r)
	}
}
