package assistant

import (
	"context"
	"strconv"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

type taskWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type taskRow struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Status  string     `json:"status"`
	Done    bool       `json:"done"`
	DueDate *time.Time `json:"dueDate"`
}

// TaskSource reads open tasks through the opus CLI.
type TaskSource struct {
	Run      Runner
	Settings settings.Assistant
	Timeout  time.Duration
}

// Fetch returns the open tasks of every picked workspace. Done tasks are left
// out.
func (t *TaskSource) Fetch(ctx context.Context) ([]Item, error) {
	var spaces []taskWorkspace
	if err := runJSON(ctx, t.Run, t.Timeout, &spaces, "opus", "workspace", "list", "--json"); err != nil {
		return nil, err
	}
	var items []Item
	for _, ws := range spaces {
		n, err := strconv.Atoi(ws.ID)
		if err != nil || !pickedID(t.Settings.Sources.Workspaces, n) {
			continue
		}
		got, err := t.tasks(ctx, ws)
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
	}
	return items, nil
}

func (t *TaskSource) tasks(ctx context.Context, ws taskWorkspace) ([]Item, error) {
	var rows []taskRow
	if err := runJSON(ctx, t.Run, t.Timeout, &rows, "opus", "-w", ws.ID, "task", "list", "--json"); err != nil {
		return nil, err
	}
	var items []Item
	for _, r := range rows {
		if r.Done || r.Status == "done" {
			continue
		}
		it := Item{Source: SourceTask, ID: r.ID, Title: r.Title, Account: ws.Name, Status: r.Status}
		if r.DueDate != nil {
			it.Due = *r.DueDate
		}
		items = append(items, it)
	}
	return items, nil
}
