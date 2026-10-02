package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/bulle/internal/toolview"
)

// green, red and yellow are the three run-state colours a parallel task row's
// marker, tool glyph and activity label share with the transcript: green for
// succeeded, red for failed, and yellow while still running. The tool's own
// tone stays on the tool name in every state.
var green = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
var red = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
var yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

// parallelMarker is the leading glyph of a task row: a yellow spinner while the
// task runs, a green check when it finished cleanly, a red cross when it failed.
func (m *Model) parallelMarker(pt parallelTaskInfo) string {
	if pt.Active {
		return yellow.Render(m.spin.View())
	}
	if pt.Err != "" {
		return red.Render("✗")
	}
	return green.Render("✓")
}

// parallelGlyph renders the task's current activity after the title. While the
// task runs a tool, the tool's glyph carries the call's state — yellow while it
// is in flight, green the moment it succeeds, red when it fails — and the tool
// name keeps the tool's own tone either way. Between tool calls the row shows
// the agent's instead: thinking once tokens have streamed, waiting before the
// first one. Empty when the task is finished.
func (m *Model) parallelGlyph(pt parallelTaskInfo) string {
	if pt.Tool != "" {
		icon := yellow
		if pt.ToolOut == "ok" {
			icon = green
		} else if pt.ToolOut != "" {
			icon = red
		}
		name := toolview.ToolTone(pt.Tool).Render(" " + pt.Tool)
		return icon.Render(toolview.ToolGlyph(pt.Tool)) + name
	}
	if !pt.Active {
		return ""
	}
	if pt.Usage.Total() > 0 {
		return yellow.Render("✻") + " " + m.theme.Muted.Render("thinking")
	}
	return yellow.Render("…") + " " + m.theme.Muted.Render("waiting")
}
