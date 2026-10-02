package tui

import (
	"context"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/diagnostics"
)

type startupDiagnostics string

var filetConfigs = []string{"filet.yml", ".filet.yml"}

func hasFiletConfig(root string) bool {
	for _, name := range filetConfigs {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}
	return false
}

func checkStartupDiagnostics(root string) tea.Cmd {
	return func() tea.Msg {
		text, err := diagnostics.Run(context.Background(), root, false)
		if err != nil {
			return startupDiagnostics("")
		}
		return startupDiagnostics(text)
	}
}

// startupDiagnostics arms the launch sweep of the opened root, but only when
// the diagnostics loop is on and filet is configured there — a repo with no
// filet.yml has no checker to report about, and silence is the ordinary case.
func (m *Model) startupDiagnostics() tea.Cmd {
	if !m.diagLoop || !hasFiletConfig(m.run.root) {
		return nil
	}
	return checkStartupDiagnostics(m.run.root)
}

func (m *Model) recordStartupDiagnostics(note startupDiagnostics) tea.Cmd {
	if note == "" {
		return nil
	}
	m.say(fromClient, string(note))
	return nil
}
