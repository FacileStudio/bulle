package assistant

import "time"

// Snooze silences key until the given time, then lets it surface once more.
func (l *Ledger) Snooze(key string, now, until time.Time) error {
	return l.update(func() {
		l.touch(key, now).SnoozedUntil = until
	})
}

// Unsuppressed drops the items that are snoozed at now or marked as noise.
// An item never surfaced, or already surfaced and unchanged, stays.
func (l *Ledger) Unsuppressed(items []Item, now time.Time) []Item {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refresh()
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if e := l.entries[it.Key()]; e != nil && (e.Noise || now.Before(e.SnoozedUntil)) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// SnoozeUntilTomorrow silences key until the next 09:00 in loc.
func (l *Ledger) SnoozeUntilTomorrow(key string, now time.Time, loc *time.Location) error {
	return l.Snooze(key, now, nextNineAM(now, loc))
}
