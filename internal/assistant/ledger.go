package assistant

import "time"

const pruneAfter = 30 * 24 * time.Hour

// ShouldSurface is true when the item is new, its fingerprint changed since
// it was last surfaced, or it newly crossed a threshold (event within 60
// minutes, task due today, task overdue). It is false when the item is
// snoozed or marked as noise.
func (l *Ledger) ShouldSurface(item Item, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[item.Key()]
	if e == nil {
		return true
	}
	if e.Noise || now.Before(e.SnoozedUntil) {
		return false
	}
	if e.LastSurfaced.IsZero() || e.Fingerprint != item.Fingerprint() {
		return true
	}
	phase := phaseOf(item, now)
	return phase != "" && phase != e.Phase
}

func (l *Ledger) touch(key string, now time.Time) *Entry {
	e := l.entries[key]
	if e == nil {
		e = &Entry{FirstSeen: now}
		l.entries[key] = e
	}
	e.LastSeen = now
	return e
}

// MarkSurfaced records that the items were told to the user.
func (l *Ledger) MarkSurfaced(items []Item, now time.Time) error {
	return l.update(func() {
		for _, it := range items {
			e := l.touch(it.Key(), now)
			e.LastSurfaced = now
			e.Fingerprint = it.Fingerprint()
			e.LastStatus = it.Status
			e.Phase = phaseOf(it, now)
		}
	})
}

// Seen records first_seen for items that were looked at but not surfaced.
func (l *Ledger) Seen(items []Item, now time.Time) error {
	return l.update(func() {
		for _, it := range items {
			l.touch(it.Key(), now)
		}
	})
}

// Snooze silences key until the given time.
func (l *Ledger) Snooze(key string, until time.Time) error {
	return l.update(func() {
		l.touch(key, time.Now()).SnoozedUntil = until
	})
}

// SnoozeUntilTomorrow silences key until the next 09:00 in loc.
func (l *Ledger) SnoozeUntilTomorrow(key string, now time.Time, loc *time.Location) error {
	return l.update(func() {
		l.touch(key, now).SnoozedUntil = nextNineAM(now, loc)
	})
}

// MarkNoise silences key for good.
func (l *Ledger) MarkNoise(key string) error {
	return l.update(func() {
		l.touch(key, time.Now()).Noise = true
	})
}

// Prune removes entries not seen for 30 days.
func (l *Ledger) Prune(now time.Time) error {
	return l.update(func() {
		for k, e := range l.entries {
			if now.Sub(e.LastSeen) > pruneAfter {
				delete(l.entries, k)
			}
		}
	})
}
