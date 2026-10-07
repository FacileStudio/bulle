package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/layout"
	"github.com/FacileStudio/bulle/internal/theme"
)

type screen struct {
	width        int
	windowHeight int
	liveRows     int
	frameRows    int
	mode         int
	transparent  bool
}

func (m *Model) View() tea.View {
	view := m.assembleView()
	return view
}

func (m *Model) aboveContent() []string {
	live := m.streaming()
	var above []string
	if len(live) > 0 {
		above = append(above, "")
	}
	above = append(above, live...)
	above = append(above, "")
	if tasksView := strings.Join(m.tasks.View(max(m.width-2, 1), m.theme.Muted), "\n"); tasksView != "" {
		above = append(above, tasksView)
		above = append(above, "")
	}
	above = append(above, m.status())
	above = append(above, "")
	above = append(above, strings.Join(m.Queue.View(m.hist.Editing(m.Len()), max(m.width-2, 1), m.theme.Question), "\n"))
	return m.margined(above)
}

// margined wraps every painted live row in a single space so nothing touches
// the left or right screen edge. The renderers fill the width they are given,
// so aboveContent hands the queue and tasks views two cells fewer and this
// pass clips a row that still came back full-width down to m.width-2 before
// the spaces go on — otherwise the margin would push every full row past the
// edge and wrap it. A row that opens with a box border glyph (▌) carries its
// own spine, so it gets only the trailing space: the pane edge is its own
// left margin, as it is in the printed transcript. Blank separator rows stay
// empty: a gutter would turn them into phantom content and break the
// blank-line bookkeeping in tuiUpper and assembleInline.
func (m *Model) margined(rows []string) []string {
	fit := make([]string, 0, len(rows))
	for _, row := range rows {
		padded := strings.Split(row, "\n")
		for i := range padded {
			if padded[i] == "" {
				continue
			}
			trimmed := layout.Shave(padded[i], max(m.width-2, 1))
			if strings.HasPrefix(layout.Unstyled(padded[i]), "▌") {
				padded[i] = trimmed + " "
				continue
			}
			padded[i] = " " + trimmed + " "
		}
		fit = append(fit, strings.Join(padded, "\n"))
	}
	return fit
}

// belowContent is everything rendered beneath the prompt: the running parallel
// parallel_agents. The slash-command menu sits above the prompt instead, so it
// never pushes this block further down the screen.
func (m *Model) belowContent() string {
	if len(m.parallelTasks) == 0 {
		return ""
	}
	return strings.Join(m.margined([]string{m.parallelTasksView()}), "\n")
}

// resize updates dimensions and reflows the transcript on width change.
func (m *Model) resize(size tea.WindowSizeMsg) tea.Cmd {
	widthChanged := size.Width != m.width
	m.width, m.windowHeight = size.Width, size.Height

	m.prompt.SetWidth(size.Width)
	m.prompt.MaxHeight = layout.PromptCap(size.Height)
	m.prompt.SetHeight(m.prompt.Height())
	m.layout(size.Height)

	if widthChanged {
		m.restyle()
		m.reflowHold()
	}
	return nil
}

// retheme swaps the palette for a terminal background-colour report and
// rebuilds the renderer, so route's dispatcher stays one line per arm.
func (m *Model) retheme(message tea.BackgroundColorMsg) tea.Cmd {
	m.theme = theme.Themed(message.IsDark())
	m.restyle()
	return nil
}

func (m *Model) layout(height int) {
	taken := 3 + m.prompt.Height() + m.menu.Height() + m.Height(m.hist.Editing(m.Len())) + m.tasks.Rows() + m.parallelTasksRows()
	m.liveRows = layout.LiveRows(height, taken)
}

// holdRowsCap is how many painted rows the alternate-screen transcript keeps.
// Every frame redraws a tail of it, so an uncapped hold would charge a long
// session more per frame for rows no frame can show. Past the cap the oldest
// rows drop ring-buffer style; the newest batch just appended always survives.
// Generous — several screens tall — so a resize to a taller terminal still
// finds history to reveal.
const holdRowsCap = 1000

func (m *Model) printed(text string) tea.Cmd {
	if m.mode == modeTUI {
		m.hold = append(m.hold, strings.Split(text, "\n")...)
		if over := len(m.hold) - holdRowsCap; over > 0 {
			m.hold = append(m.hold[:0], m.hold[over:]...)
		}
		return nil
	}
	return m.printBatches(text)
}
