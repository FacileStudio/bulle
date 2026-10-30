package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

func (a *approvals) drop(room string, card *approvalCard) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.removeLocked(room, card)
}

func (a *approvals) removeLocked(room string, card *approvalCard) {
	cards := slices.DeleteFunc(a.pending[room], func(c *approvalCard) bool { return c == card })
	if len(cards) == 0 {
		delete(a.pending, room)
		return
	}
	a.pending[room] = cards
}

func (a *approvals) settle(room string, card *approvalCard, ok bool) {
	a.removeLocked(room, card)
	card.reply <- ok
}

func reactionVerdict(emoji string) (approve, known bool) {
	switch strings.ReplaceAll(strings.TrimSpace(emoji), "️", "") {
	case "✅":
		return true, true
	case "❌":
		return false, true
	}
	return false, false
}

func textVerdict(text string) (approve, known bool) {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(text), ".!")) {
	case "oui", "yes", "ok", "y":
		return true, true
	case "non", "no", "n":
		return false, true
	}
	return false, false
}

// HandleReaction resolves the pending card a reaction points at. It reports
// whether the reaction was an approval verdict on a known card, so an unknown
// event or an unrelated emoji falls through untouched.
func (a *approvals) HandleReaction(_ context.Context, m chat.Message, _ []string) bool {
	verdict, known := reactionVerdict(m.Reaction)
	if !known || m.Target == "" {
		return false
	}
	room := m.Key()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, card := range a.pending[room] {
		if card.event == m.Target {
			a.settle(room, card, verdict)
			return true
		}
	}
	return false
}

// Intercept consumes a plain text verdict while a card is pending in the
// message's room, so the model never sees it. It reports whether it consumed
// the message.
func (a *approvals) Intercept(m chat.Message) bool {
	if !m.Is(chat.KindText) {
		return false
	}
	verdict, known := textVerdict(m.Text)
	if !known {
		return false
	}
	room := m.Key()
	a.mu.Lock()
	cards := a.pending[room]
	if len(cards) == 0 {
		a.mu.Unlock()
		return false
	}
	if len(cards) > 1 {
		a.mu.Unlock()
		a.askForReaction(room)
		return true
	}
	a.settle(room, cards[0], verdict)
	a.mu.Unlock()
	return true
}

func (a *approvals) askForReaction(room string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	text := "Plusieurs validations attendent : réponds par ✅ ou ❌ directement sur la carte concernée."
	if _, err := chat.Deliver(ctx, a.adapter, roomIdentity(room), text, false); err != nil {
		fmt.Fprintf(os.Stderr, "bulle chat: approval notice: %v\n", err)
	}
}
