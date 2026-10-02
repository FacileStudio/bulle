package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/bulle/internal/layout"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/nacelle"
)

// parallelTasksView returns a view of the parallel subagent tasks, one line per
// task. A running task shows its title, the tool it is running right now, and
// an elapsed clock that the spinner tick redraws; a finished one shows its own
// spend from the result's usage map and the duration it took, so the per-subagent
// cost is visible rather than a single total copy-pasted onto every row. Tasks
// a /clear hid are not drawn.
func (m *Model) parallelTasksView() string {
	if len(m.parallelTasks) == 0 {
		return ""
	}
	var lines []string
	for _, tasks := range m.parallelTasks {
		for _, pt := range tasks {
			if pt.Cleared {
				continue
			}
			lines = append(lines, m.taskRow(pt))
		}
	}
	return strings.Join(lines, "\n")
}

// The elapsed clock keeps its own style, split from the muted spend so the
// timer reads at a glance while it ticks: yellow while the task runs, green
// once it finishes, red when it fails.

func (m *Model) taskRow(pt parallelTaskInfo) string {
	const gap = 3
	width := max(m.width-2, 1)
	marker := m.parallelMarker(pt)
	tool := ""
	if glyph := m.parallelGlyph(pt); glyph != "" {
		tool = " " + glyph
	}
	clock := taskTone(pt).Render(taskClock(pt))
	spend := ""
	if s := taskSpend(pt.Usage); s != "" {
		spend = m.theme.Muted.Render(s) + " "
	}
	tail := strings.TrimSpace(spend + clock)
	room := max(width-lipgloss.Width(tool)-lipgloss.Width(tail)-gap, 0)
	title := layout.Truncate(taskTitle(pt), max(room-lipgloss.Width(marker)-1, 0))
	left := marker + " " + taskTone(pt).Render(title+":")
	pad := max(width-lipgloss.Width(left)-lipgloss.Width(tool)-lipgloss.Width(tail), 0)
	return left + tool + strings.Repeat(" ", pad) + tail
}

func taskTone(pt parallelTaskInfo) lipgloss.Style {
	if !pt.Active {
		if pt.Err != "" {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
}

// taskClock is the elapsed time: live for a running task, frozen for a done one.
func taskClock(pt parallelTaskInfo) string {
	elapsed := time.Since(pt.Began)
	if !pt.Active && !pt.End.IsZero() {
		elapsed = pt.End.Sub(pt.Began)
	}
	return status.Lasted(elapsed)
}

// taskSpend formats one subagent's own token burn, empty until the result
// arrives with its per-task usage map.
func taskSpend(usage nacelle.Usage) string {
	var parts []string
	if usage.Cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.4f", usage.Cost))
	}
	if usage.Total() > 0 {
		parts = append(parts,
			"↑"+status.ShortTokens(usage.InputTokens+usage.CacheCreationTokens),
			"↓"+status.ShortTokens(usage.OutputTokens))
	}
	return strings.Join(parts, " ")
}

// parallelTasksRows returns the number of parallel task rows for layout.
func (m *Model) parallelTasksRows() int {
	return parallelTaskRows(m.parallelTasks)
}

// hasLiveParallel reports whether any subagent task is still running, so the
// spinner keeps ticking — and with it the elapsed clock and live spend keep
// redrawing — after the parent run has settled and m.run.busy went false.
func (m *Model) hasLiveParallel() bool {
	for _, tasks := range m.parallelTasks {
		for _, pt := range tasks {
			if pt.Active {
				return true
			}
		}
	}
	return false
}
