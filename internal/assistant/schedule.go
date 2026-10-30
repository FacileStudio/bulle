package assistant

import (
	"slices"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

// Len returns the number of ledger entries.
func (l *Ledger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// NextDigest returns the next time the named digest runs after now, skipping
// weekend days the digest does not run on. The zero time means it never runs.
func NextDigest(a settings.Assistant, name string, now time.Time) time.Time {
	a = a.WithDefaults()
	mins, ok := parseClock(a.Digests[name])
	if !ok {
		return time.Time{}
	}
	loc, err := time.LoadLocation(a.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	for i := range 8 {
		y, m, d := local.Date()
		at := time.Date(y, m, d+i, mins/60, mins%60, 0, 0, loc)
		if at.After(now) && digestDay(a, name, at) {
			return at
		}
	}
	return time.Time{}
}

func digestDay(a settings.Assistant, name string, at time.Time) bool {
	day := at.Weekday()
	if day != time.Saturday && day != time.Sunday {
		return true
	}
	return slices.Contains(a.WeekendDigests, name)
}
