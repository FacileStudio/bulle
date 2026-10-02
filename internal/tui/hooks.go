package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/toolview"
)

type hookReportMsg struct {
	report settings.HookReport
}

func watchHooks() tea.Cmd {
	return func() tea.Msg {
		r, ok := <-settings.HookReports()
		if !ok {
			return nil
		}
		return hookReportMsg{report: r}
	}
}

func (m *Model) recordHook(msg hookReportMsg) tea.Cmd {
	if !m.showHooks {
		return watchHooks()
	}
	r := msg.report
	line := toolview.HookLine(string(r.Event), r.Tool, r.Command, r.Label, m.width)
	styled := toolview.ColorGlyph(line, "36", "39") + " · " + took(r.Duration)
	m.say(fromTool, styled)
	m.recordHookStatus(r)
	m.recordHookOutput(r)
	return watchHooks()
}

func (m *Model) recordHookStatus(r settings.HookReport) {
	if r.Denied {
		reason := r.Reason
		if reason == "" {
			reason = "denied by hook"
		}
		m.say(fromResult, "denied: "+reason)
		return
	}
	if r.Err != nil && r.ExitCode != 0 && r.ExitCode != 2 {
		m.say(fromResult, fmt.Sprintf("hook failed: %v", r.Err))
	}
}

func (m *Model) recordHookOutput(r settings.HookReport) {
	if !m.showHookOutput {
		return
	}
	out := r.Stdout
	if out == "" && r.Stderr != "" && !r.Denied {
		out = r.Stderr
	}
	if preview := toolview.HookOutputPreview(out, 6, m.width); preview != "" {
		m.say(fromResult, preview)
	}
}
