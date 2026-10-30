package assistant

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

// WatchResult describes one watch run. Delivery is empty when nothing was
// worth sending.
type WatchResult struct {
	Surfaced    []Item
	ModelCalled bool
	Delivery    string
}

// Watch fetches mail, calendar and tasks, keeps what trips a hard trigger and
// the ledger lets through, and sends one interrupt. It never calls the model.
func Watch(ctx context.Context, d Deps) (WatchResult, error) {
	d, err := d.withDefaults()
	if err != nil {
		return WatchResult{}, err
	}
	unlock, err := AcquireRun(d.StateDir)
	if errors.Is(err, ErrLocked) {
		return WatchResult{Delivery: DeliverySuppressed}, nil
	}
	if err != nil {
		return WatchResult{}, err
	}
	defer unlock()
	return watchLocked(ctx, d)
}

func watchLocked(ctx context.Context, d Deps) (WatchResult, error) {
	now := d.Now()
	if InQuiet(now, d.Settings, d.StateDir) {
		return WatchResult{Delivery: DeliverySuppressed}, nil
	}
	if err := d.Ledger.Prune(now); err != nil {
		fmt.Fprintf(d.Log, "assistant: prune ledger: %v\n", err)
	}
	items, ferr := fetchAll(ctx, d, sinceArg(LastWatch(d.StateDir), now))
	if ferr != nil && len(items) == 0 {
		return WatchResult{}, ferr
	}
	if ferr != nil {
		fmt.Fprintf(d.Log, "assistant: partial fetch: %v\n", ferr)
	}
	res, err := interrupt(ctx, d, items, now.In(d.location()))
	if err != nil {
		return res, err
	}
	if ferr == nil && res.Delivery != DeliveryFailed {
		err = saveLastWatch(d.StateDir, now)
	}
	return res, err
}

func interrupt(ctx context.Context, d Deps, items []Item, now time.Time) (WatchResult, error) {
	hits := hardTriggers(d, items, now)
	res := WatchResult{Surfaced: hits}
	if len(hits) > 0 {
		keys := itemKeys(hits)
		yes := true
		res.Delivery = deliver(ctx, d, chat.IPCRequest{
			Action: "send", Text: interruptText(hits, now, d.english()),
			Notify: &yes, Record: true, Items: keys,
		})
		if res.Delivery == DeliveryOK {
			if err := d.Ledger.MarkSurfaced(hits, now); err != nil {
				return res, err
			}
		}
	}
	return res, d.Ledger.Seen(items, now)
}

func hardTriggers(d Deps, items []Item, now time.Time) []Item {
	var hits []Item
	for _, it := range items {
		if isHard(it, now) && d.Ledger.ShouldSurface(it, now) {
			hits = append(hits, it)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return rank(hits[i]) < rank(hits[j]) })
	return hits
}

func isHard(it Item, now time.Time) bool {
	switch it.Source {
	case SourceCalendar:
		return phaseOf(it, now) == "soon"
	case SourceTask:
		p := phaseOf(it, now)
		return p == "today" || p == "overdue"
	case SourceMail:
		return it.Class == ClassVIP || it.Class == ClassKnown
	}
	return false
}

func rank(it Item) int {
	switch it.Source {
	case SourceCalendar:
		return 0
	case SourceTask:
		return 1
	}
	return 2
}
