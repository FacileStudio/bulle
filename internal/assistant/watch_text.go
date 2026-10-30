package assistant

import (
	"fmt"
	"strings"
	"time"
)

func interruptText(items []Item, now time.Time, en bool) string {
	lines := make([]string, len(items))
	for i, it := range items {
		lines[i] = itemLine(it, now, en)
	}
	if len(lines) == 1 {
		return lines[0]
	}
	head := "À signaler :"
	if en {
		head = "Heads up:"
	}
	return head + "\n- " + strings.Join(lines, "\n- ")
}

func itemLine(it Item, now time.Time, en bool) string {
	switch it.Source {
	case SourceCalendar:
		return eventLine(it, now, en)
	case SourceTask:
		return taskLine(it, now, en)
	}
	if en {
		return fmt.Sprintf("New mail from %s: %s", it.Who, it.Title)
	}
	return fmt.Sprintf("Nouveau mail de %s : %s", it.Who, it.Title)
}

func eventLine(it Item, now time.Time, en bool) string {
	mins := int(it.When.Sub(now).Round(time.Minute).Minutes())
	if en {
		return fmt.Sprintf("In %d min: %s", mins, it.Title)
	}
	return fmt.Sprintf("Dans %d min : %s", mins, it.Title)
}

func taskLine(it Item, now time.Time, en bool) string {
	late := phaseOf(it, now) == "overdue"
	switch {
	case en && late:
		return "Overdue task: " + it.Title
	case en:
		return "Task due today: " + it.Title
	case late:
		return "Tâche en retard : " + it.Title
	}
	return "Tâche pour aujourd'hui : " + it.Title
}
