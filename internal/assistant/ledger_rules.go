package assistant

import "time"

const soonWindow = 60 * time.Minute

func phaseOf(item Item, now time.Time) string {
	switch item.Source {
	case SourceCalendar:
		if d := item.When.Sub(now); !item.When.IsZero() && d >= 0 && d <= soonWindow {
			return "soon"
		}
	case SourceTask:
		return taskPhase(item.Due, now)
	}
	return ""
}

func taskPhase(due, now time.Time) string {
	if due.IsZero() {
		return ""
	}
	dy, dm, dd := due.Date()
	ny, nm, nd := now.Date()
	dueDay := time.Date(dy, dm, dd, 0, 0, 0, 0, time.UTC)
	today := time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC)
	switch {
	case dueDay.Before(today):
		return "overdue"
	case dueDay.Equal(today):
		return "today"
	}
	return ""
}

func nextNineAM(now time.Time, loc *time.Location) time.Time {
	y, m, d := now.In(loc).Date()
	at := time.Date(y, m, d, 9, 0, 0, 0, loc)
	if !at.After(now) {
		at = time.Date(y, m, d+1, 9, 0, 0, 0, loc)
	}
	return at
}
