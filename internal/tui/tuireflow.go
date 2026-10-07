package tui

import (
	"strings"

	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/bulle/internal/toolview"
)

const holdEntriesCap = 1000

func (m *Model) recordHeld(who speaker, text string) {
	m.held = append(m.held, heldEntry{who: who, text: text})
	if over := len(m.held) - holdEntriesCap; over > 0 {
		m.held = append(m.held[:0], m.held[over:]...)
		clear(m.held[len(m.held):cap(m.held)])
	}
}

func (m *Model) renderHeld(width int) []string {
	if len(m.held) == 0 {
		return nil
	}
	m.width = width
	painted := make([]string, len(m.held))
	for i, entry := range m.held {
		painted[i] = strings.Trim(m.paintEntry(entry, width), "\n")
	}
	var sb strings.Builder
	sb.WriteString(painted[0])
	prev := painted[0]
	for _, e := range painted[1:] {
		sb.WriteString(m.gap(prev, e))
		sb.WriteString(e)
		prev = e
	}
	rows := strings.Split(sb.String(), "\n")
	if over := len(rows) - holdRowsCap; over > 0 {
		rows = append(rows[:0], rows[over:]...)
	}
	return rows
}

func (m *Model) paintEntry(entry heldEntry, width int) string {
	if entry.who == fromDiff {
		if rebuilt := m.rebuildDiff(entry, width); rebuilt != "" {
			return rebuilt
		}
	}
	return m.paint(entry.who, entry.text)
}

func (m *Model) rebuildDiff(entry heldEntry, width int) string {
	if entry.diff == nil && entry.out == "" {
		return ""
	}
	var box strings.Builder
	if entry.diff != nil {
		if d := diff.RenderDiff(*entry.diff, width, boxBorder(entry.ok), m.theme.Muted, m.transparent); d != "" {
			box.WriteString(d)
		}
	}
	if entry.out != "" {
		box.WriteString(m.outputBox(entry.out, entry.ok))
	}
	return box.String()
}

func (m *Model) reflowHold() {
	if m.mode == modeTUI && len(m.held) > 0 {
		m.hold = m.renderHeld(m.width)
		return
	}
	limit := max(m.width-1, 1)
	reflowed := make([]string, 0, len(m.hold))
	for _, row := range m.hold {
		reflowed = append(reflowed, toolview.WrapRow(row, limit)...)
	}
	m.hold = reflowed
}
