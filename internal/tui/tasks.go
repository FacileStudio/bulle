package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/tasks"
)

// watchTasks re-arms the watcher that brings plan updates in from the task
// tool's goroutine.
func watchTasks() tea.Cmd {
	return func() tea.Msg {
		return <-tasks.ReportChan()
	}
}

// recordTasks stores the latest plan the task tool reported, re-lays the
// footer that shows it, and keeps watching.
func (m *Model) recordTasks(reported tasks.TaskUpdate) tea.Cmd {
	m.tasks = tasks.TaskList(reported)
	tasks.SetCurrentPlan(m.tasks)
	m.layout(m.windowHeight)
	return watchTasks()
}
