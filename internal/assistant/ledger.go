package assistant

import "time"

const pruneAfter = 30 * 24 * time.Hour

// ShouldSurface is true when the item is new, its snooze expired unanswered,
// its fingerprint changed since it was last surfaced, or it newly crossed a
// threshold (event within 60 minutes, task due today, task overdue). It is
// false when the item is snoozed or marked as noise. It reads the file again
// so a change made by another process is seen.
func (l *Ledger) ShouldSurface(item Item, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refresh()
	e := l.entries[item.Key()]
	if e == nil {
		return true
	}
	if e.Noise || now.Before(e.SnoozedUntil) {
		return false
	}
	if snoozeExpired(e) || e.LastSurfaced.IsZero() || e.Fingerprint != item.Fingerprint() {
		return true
	}
	phase := phaseOf(item, now)
	return phase != "" && phase != e.Phase
}

func snoozeExpired(e *Entry) bool {
	return !e.SnoozedUntil.IsZero() && e.LastSurfaced.Before(e.SnoozedUntil)
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
