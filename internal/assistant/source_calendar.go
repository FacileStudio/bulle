package assistant

import (
	"context"
	"strconv"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

type calendarRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type calendarEvent struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	StartAt time.Time `json:"start_at"`
	Status  string    `json:"status"`
}

// CalendarSource reads today's events through the agenda CLI.
type CalendarSource struct {
	Run      Runner
	Settings settings.Assistant
	Timeout  time.Duration
	Now      func() time.Time
}

// Fetch returns the events starting between today and tomorrow, cancelled ones
// excluded.
func (c *CalendarSource) Fetch(ctx context.Context) ([]Item, error) {
	var cals []calendarRef
	if err := runJSON(ctx, c.Run, c.Timeout, &cals, "agenda", "calendars", "list", "--json"); err != nil {
		return nil, err
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	from := now()
	var items []Item
	for _, cal := range cals {
		if !pickedID(c.Settings.Sources.Calendars, cal.ID) {
			continue
		}
		got, err := c.events(ctx, cal, from)
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
	}
	return items, nil
}

func (c *CalendarSource) events(ctx context.Context, cal calendarRef, from time.Time) ([]Item, error) {
	var evs []calendarEvent
	args := []string{
		"agenda", "events", "list", "--calendar", strconv.Itoa(cal.ID),
		"--from", from.Format("2006-01-02"),
		"--to", from.AddDate(0, 0, 1).Format("2006-01-02"), "--json",
	}
	if err := runJSON(ctx, c.Run, c.Timeout, &evs, args...); err != nil {
		return nil, err
	}
	var items []Item
	for _, e := range evs {
		if e.Status == "cancelled" {
			continue
		}
		items = append(items, Item{
			Source:  SourceCalendar,
			ID:      strconv.Itoa(e.ID),
			Title:   e.Title,
			Account: cal.Name,
			When:    e.StartAt,
			Status:  e.Status,
		})
	}
	return items, nil
}

func pickedID(ids []int, id int) bool {
	if len(ids) == 0 {
		return true
	}
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
