package queue

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/bulle/internal/layout"
)

// Replace updates the message at index.
func (q *Queue) Replace(i int, text string) {
	q.items[i] = text
}

// Editing returns the index in queued being edited, or -1 if none.
func (q *Queue) Editing(fromEnd int) int {
	if fromEnd <= 0 || fromEnd > len(q.items) {
		return -1
	}
	return len(q.items) - fromEnd
}

// Waiting returns the queue minus the line being edited.
func (q *Queue) Waiting(editing int) []string {
	if editing < 0 || editing >= len(q.items) {
		return q.items
	}
	waiting := make([]string, 0, len(q.items)-1)
	return append(append(waiting, q.items[:editing]...), q.items[editing+1:]...)
}

// Height returns how many terminal rows View draws.
func (q *Queue) Height(editing int) int {
	waiting := len(q.Waiting(editing))
	if waiting <= MaxRows {
		return waiting
	}
	return MaxRows + 1
}

// NextToSend returns the index of the first message not currently being edited.
func (q *Queue) NextToSend(editing int) int {
	for i := range q.items {
		if i != editing {
			return i
		}
	}
	return -1
}

// View returns rendered strings for waiting messages. Every line carries the
// same left half-block spine (▌) the tool boxes and prompt draw, one cell and
// one space ahead of the text, so the queue reads as a bordered pane. The
// passing-in style carries the tone; the spine gets no block background of its
// own.
func (q *Queue) View(editing, width int, style lipgloss.Style) []string {
	shown, hidden := q.Waiting(editing), 0
	if len(shown) > MaxRows {
		shown, hidden = shown[:MaxRows], len(shown)-MaxRows
	}

	w := max(width-2, 0)
	lines := make([]string, 0, q.Height(editing))
	for _, text := range shown {
		lines = append(lines, style.Width(width).Render("▌ "+layout.Truncate(layout.Unstyled(text), w)))
	}
	if hidden > 0 {
		lines = append(lines, style.Width(width).Render("▌ "+layout.Truncate(fmt.Sprintf("and %d more", hidden), w)))
	}
	return lines
}
