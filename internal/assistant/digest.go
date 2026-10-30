package assistant

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

// DigestNames returns the digests Digest accepts.
func DigestNames() []string {
	return []string{"morning", "midday", "evening"}
}

// DigestResult describes one digest run.
type DigestResult struct {
	Surfaced    []Item
	ModelCalled bool
	Delivery    string
}

// Digest builds the payload for one digest, asks the model for the prose and
// falls back to a template when the answer is unusable. Quiet hours do not
// apply; weekends only run the names in WeekendDigests.
func Digest(ctx context.Context, d Deps, name string) (DigestResult, error) {
	if !slices.Contains(DigestNames(), name) {
		return DigestResult{}, fmt.Errorf("unknown digest %q: want morning, midday or evening", name)
	}
	d, err := d.withDefaults()
	if err != nil {
		return DigestResult{}, err
	}
	now := d.Now().In(d.location())
	if weekendSkip(d, name, now) {
		return DigestResult{Delivery: DeliverySuppressed}, nil
	}
	items, ferr := fetchAll(ctx, d, "")
	if ferr != nil && len(items) == 0 {
		return DigestResult{}, ferr
	}
	if ferr != nil {
		fmt.Fprintf(d.Log, "assistant: partial fetch: %v\n", ferr)
	}
	return sendDigest(ctx, d, buildPayload(name, items, now), now)
}

func weekendSkip(d Deps, name string, now time.Time) bool {
	day := now.Weekday()
	if day != time.Saturday && day != time.Sunday {
		return false
	}
	return !slices.Contains(d.Settings.WeekendDigests, name)
}

func sendDigest(ctx context.Context, d Deps, p Payload, now time.Time) (DigestResult, error) {
	evening := p.Digest == "evening"
	if len(p.listed) == 0 && !evening {
		return DigestResult{Delivery: DeliverySuppressed}, nil
	}
	res := DigestResult{Surfaced: p.listed}
	text := ""
	if len(p.listed) > 0 {
		text, res.ModelCalled = modelDigest(ctx, d, p)
	}
	if text == "" {
		text = templateDigest(p, d.english())
	}
	if evening {
		text = withDebrief(text)
	}
	no := false
	keys := make([]string, len(p.listed))
	for i, it := range p.listed {
		keys[i] = it.Key()
	}
	res.Delivery = deliver(ctx, d, chat.IPCRequest{
		Action: "send", Text: text, Notify: &no, Record: true, Items: keys,
	})
	if res.Delivery == DeliveryOK {
		return res, d.Ledger.MarkSurfaced(p.listed, now)
	}
	return res, nil
}
