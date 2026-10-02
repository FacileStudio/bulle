package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/layout"
)

// printBatches splits one painted block into the tea.Println batches inline
// mode prints, cut to the rows the frame budget can still spend — see
// printed for the alternate-screen half that holds instead.
func (m *Model) printBatches(text string) tea.Cmd {
	budget := layout.Budget(m.windowHeight, m.frameRows)
	batches := layout.Batches(text, budget, m.width)
	cmds := make([]tea.Cmd, 0, len(batches))
	for _, batch := range batches {
		cmds = append(cmds, tea.Println(batch))
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Sequence(cmds...)
}
