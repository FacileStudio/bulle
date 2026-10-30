package assistant

import (
	"context"
	"errors"
	"fmt"
)

func fetchAll(ctx context.Context, d Deps, since string) ([]Item, error) {
	mail := &MailSource{Run: d.Run, Settings: d.Settings, Since: since, Log: d.Log, StateDir: d.StateDir}
	cal := &CalendarSource{Run: d.Run, Settings: d.Settings, Now: d.Now}
	tasks := &TaskSource{Run: d.Run, Settings: d.Settings}
	fetchers := []struct {
		name string
		do   func(context.Context) ([]Item, error)
	}{{"mail", mail.Fetch}, {"calendar", cal.Fetch}, {"tasks", tasks.Fetch}}
	var items []Item
	var errs []error
	for _, f := range fetchers {
		got, err := f.do(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.name, err))
			continue
		}
		items = append(items, got...)
	}
	return items, errors.Join(errs...)
}
