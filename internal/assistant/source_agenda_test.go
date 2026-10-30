package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestCalendarMapping(t *testing.T) {
	f := newFake(t, map[string]string{
		"agenda calendars list --json":                                             "calendars.json",
		"agenda events list --calendar 1 --from 2026-10-30 --to 2026-10-31 --json": "events_calendar1.json",
		"agenda events list --calendar 2 --from 2026-10-30 --to 2026-10-31 --json": "[]",
	})
	now := func() time.Time { return time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC) }
	src := &CalendarSource{Run: f.run, Now: now}
	items, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (cancelled dropped): %+v", len(items), items)
	}
	ev := items[0]
	want := time.Date(2026, 10, 30, 19, 0, 0, 0, time.UTC)
	if ev.Source != SourceCalendar || ev.ID != "37" || ev.Title != "Theatre class" ||
		ev.Account != "Personal" || !ev.When.Equal(want) || ev.Status != "confirmed" {
		t.Errorf("bad mapping: %+v", ev)
	}
}

func TestCalendarRespectsSources(t *testing.T) {
	f := newFake(t, map[string]string{
		"agenda calendars list --json":                                             "calendars.json",
		"agenda events list --calendar 2 --from 2026-10-30 --to 2026-10-31 --json": "[]",
	})
	now := func() time.Time { return time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC) }
	src := &CalendarSource{
		Run:      f.run,
		Now:      now,
		Settings: settings.Assistant{Sources: settings.AssistantSources{Calendars: []int{2}}},
	}
	if _, err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 {
		t.Errorf("calls: %v", f.calls)
	}
}

var taskFiles = map[string]string{
	"opus workspace list --json": "workspaces.json",
	"opus -w 1 task list --json": "tasks_ws1.json",
	"opus -w 3 task list --json": "tasks_ws3.json",
}

func checkTaskIDs(t *testing.T, items []Item, ids []string) {
	t.Helper()
	if len(items) != len(ids) {
		t.Fatalf("got %+v, want ids %v", items, ids)
	}
	for i, id := range ids {
		if items[i].ID != id || items[i].Source != SourceTask {
			t.Errorf("item %d: %+v", i, items[i])
		}
	}
}

func TestTaskMapping(t *testing.T) {
	cases := []struct {
		name string
		ws   []int
		ids  []string
	}{
		{"all workspaces, done dropped", nil, []string{"28", "51", "90"}},
		{"one workspace", []int{3}, []string{"90"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &TaskSource{
				Run:      newFake(t, taskFiles).run,
				Settings: settings.Assistant{Sources: settings.AssistantSources{Workspaces: c.ws}},
			}
			items, err := src.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			checkTaskIDs(t, items, c.ids)
		})
	}
}

func TestTaskDueAndStatus(t *testing.T) {
	items, err := (&TaskSource{Run: newFake(t, taskFiles).run}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC)
	if !items[0].Due.Equal(due) || items[0].Status != "to-do" || items[0].Account != "Shared" {
		t.Errorf("bad mapping: %+v", items[0])
	}
	if !items[1].Due.IsZero() || items[1].Status != "in-progress" {
		t.Errorf("missing due should stay zero: %+v", items[1])
	}
}
