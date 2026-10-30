package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

type slowAdapter struct {
	cardAdapter
	release chan struct{}
}

func (s *slowAdapter) SendEvent(ctx context.Context, to chat.Identity, text string) (string, error) {
	<-s.release
	return s.cardAdapter.SendEvent(ctx, to, text)
}

func TestApprovalNotAnswerableBeforeDelivery(t *testing.T) {
	ad := &slowAdapter{cardAdapter: cardAdapter{sent: make(chan string, 4)}, release: make(chan struct{})}
	a := newApprovals(ad, chat.NewReactionRouter())
	done := make(chan askResult, 1)
	go func() {
		ok, err := a.Ask(context.Background(), "matrix:!r:s", "x")
		done <- askResult{ok, err}
	}()
	time.Sleep(50 * time.Millisecond)
	if a.Intercept(reply("ok")) {
		t.Fatal("verdict consumed before the card was delivered")
	}
	close(ad.release)
	<-ad.sent
	waitBound(t, a)
	if !a.Intercept(reply("ok")) {
		t.Fatal("verdict ignored after delivery")
	}
	if r := result(t, done); !r.ok {
		t.Fatal("not approved")
	}
}

func TestApprovalTextRefusedWithTwoCards(t *testing.T) {
	a, ad, _ := newTestApprovals()
	first := startAsk(t, a, ad)
	second := startAsk(t, a, ad)
	if !a.Intercept(reply("oui")) {
		t.Fatal("text must be consumed with two cards pending")
	}
	ad.mu.Lock()
	texts := len(ad.texts)
	ad.mu.Unlock()
	if texts != 1 {
		t.Fatalf("texts = %d, want one reminder", texts)
	}
	select {
	case r := <-first:
		t.Fatalf("first card resolved by text: %+v", r)
	case r := <-second:
		t.Fatalf("second card resolved by text: %+v", r)
	case <-time.After(50 * time.Millisecond):
	}
	a.HandleReaction(context.Background(), react("✅", "$card"), nil)
	a.HandleReaction(context.Background(), react("✅", "$card"), nil)
	result(t, first)
	result(t, second)
}
