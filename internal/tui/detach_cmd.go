package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/FacileStudio/bulle/internal/herdr"
	"github.com/FacileStudio/nacelle"
)

func (m *Model) detachCmd() tea.Cmd {
	m.detached = true
	var id string
	if m.session != nil && m.session.Path() != "" {
		id = strings.TrimSuffix(filepath.Base(m.session.Path()), ".jsonl")
	}
	if id != "" {
		m.detachedMsg = fmt.Sprintf("detached session %s\nrunning in background — resume with: bulle sessions attach %s", id, id)
	} else {
		m.detachedMsg = "session detached — running in background\nresume with: bulle sessions list"
	}
	return tea.Quit
}

func messageText(msg nacelle.Message) string {
	var sb strings.Builder
	for _, part := range msg.Parts {
		if text, ok := part.(nacelle.Text); ok {
			sb.WriteString(text.Text)
		}
	}
	return sb.String()
}

func drainDetached(m *Model) {
	if !m.run.busy || m.run.results == nil {
		return
	}
	for next := range m.run.results {
		if next.err == nil {
			m.record(next.event)
			m.absorb(next.event)
		}
	}
	m.flush()
}

func finalizeLaunch(final tea.Model) {
	done, ok := final.(*Model)
	if !ok {
		return
	}
	defer herdr.Release(done.herdrClient)
	if !done.detached {
		if done.run.cancel != nil {
			done.run.cancel()
		}
		if recap := done.recap(); recap != "" {
			fmt.Println(recap)
		}
		return
	}
	if done.detachedMsg != "" {
		fmt.Println(done.detachedMsg)
	}
	drainDetached(done)
}
