package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// A shrink mid-run re-wraps held rows at the new width so nothing is clipped,
// and a row that opened with a box border keeps that border on every
// continuation — the terminal reflows only lines it wrapped itself, so the
// hold has to do it here.
func TestResizeReflowsHeldRows(t *testing.T) {
	m := sized()
	m.hold = []string{"▌ " + strings.Repeat("word ", 30), strings.Repeat("x", 100), "short"}
	m.resize(tea.WindowSizeMsg{Width: 30, Height: 24})
	for _, row := range m.hold {
		if lipgloss.Width(row) > 29 {
			t.Errorf("held row survived the shrink too wide: %q", row)
		}
	}
	var bordered, plain int
	for _, row := range m.hold {
		switch {
		case strings.HasPrefix(row, "▌ "):
			bordered++
		case strings.HasPrefix(row, "x"), row == "short":
			plain++
		}
	}
	if bordered < 2 {
		t.Errorf("bordered continuations = %d, want each continuation carrying the border", bordered)
	}
	if plain == 0 {
		t.Errorf("plain continuations vanished")
	}
}

func TestResizeReflowsHeldEntries(t *testing.T) {
	m := tuiModel()
	m.say(fromReader, "What is the meaning of life?")
	m.say(fromModel, "42 is the answer to everything.")
	m.prints()
	m.resize(tea.WindowSizeMsg{Width: 40, Height: 24})
	if len(m.held) != 3 {
		t.Fatalf("held = %d entries, want 3", len(m.held))
	}
	for _, row := range m.hold {
		if lipgloss.Width(row) > 40 {
			t.Errorf("row %q width = %d, want <= 40", row, lipgloss.Width(row))
		}
	}
}

func TestRecordHeldCapsAtLimit(t *testing.T) {
	m := tuiModel()
	for i := range holdEntriesCap + 50 {
		m.say(fromClient, strings.Repeat("a", i+1))
	}
	if len(m.held) != holdEntriesCap {
		t.Fatalf("held = %d entries, want capped at %d", len(m.held), holdEntriesCap)
	}
}

func TestResizeReflowsWider(t *testing.T) {
	m := sized()
	m.mode = modeTUI
	m.resize(tea.WindowSizeMsg{Width: 40, Height: 24})
	m.say(fromReader, "What is the best practice for terminal resize reflow?")
	m.say(fromModel, "Terminal emulators do not reflow text automatically when wider unless the application re-renders the text from the source buffer. In alternate screen mode, bulle now retains the raw transcript and paints it afresh.")
	m.prints()
	for _, row := range m.hold {
		if lipgloss.Width(row) > 40 {
			t.Errorf("held row wider than 40: %d: %q", lipgloss.Width(row), row)
		}
	}
	m.resize(tea.WindowSizeMsg{Width: 100, Height: 24})
	var expanded bool
	for _, row := range m.hold {
		if lipgloss.Width(row) > 50 {
			expanded = true
			break
		}
	}
	if !expanded {
		t.Errorf("expected some held rows > 50 chars after widening, got: %v", m.hold)
	}
}

func TestResizeReflowsIdempotent(t *testing.T) {
	m := sized()
	m.mode = modeTUI
	m.width = 80
	m.windowHeight = 24
	m.say(fromReader, "What is the best practice for terminal resize reflow?")
	m.say(fromModel, "Terminal emulators do not reflow text automatically when wider unless the application re-renders the text from the source buffer. In alternate screen mode, bulle now retains the raw transcript and paints it afresh.")
	m.prints()

	pristine := append([]string(nil), m.hold...)

	m.resize(tea.WindowSizeMsg{Width: 35, Height: 24})
	m.resize(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.resize(tea.WindowSizeMsg{Width: 35, Height: 24})
	m.resize(tea.WindowSizeMsg{Width: 80, Height: 24})

	if !slices.Equal(m.hold, pristine) {
		t.Errorf("reflow not idempotent:\ngot  %q\nwant %q", m.hold, pristine)
	}
}

func TestClearResetsHeldEntries(t *testing.T) {
	m := tuiModel()
	m.say(fromReader, "hello")
	m.say(fromModel, "world")
	m.clear()
	if len(m.held) != 1 {
		t.Fatalf("held = %d entries after clear, want 1", len(m.held))
	}
}
