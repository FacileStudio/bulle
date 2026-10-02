package tui

import (
	"strings"

	"github.com/FacileStudio/bulle/internal/menu"
)

// commandFilter returns the slash command the autocomplete should filter on:
// the slash word under the cursor when there is one, else the first /command
// in the line. Basing it on the cursor is what lets a second /command typed
// after an already-completed skill (say `/facile-review /mus`) complete on
// its own, instead of always re-matching the skill at the front of the line.
// The fallback keeps typing straight past a command (`run /cl now`) from
// closing the dropdown, mirroring the old first-slash behaviour.
func (m *Model) commandFilter() string {
	w := m.cursorWordAt()
	if strings.HasPrefix(w.text, "/") {
		return w.text
	}
	return menu.AnyCommand(m.prompt.Value())
}

// cursorWord is the word under the prompt cursor, bounded by the menu's word
// separators (space, tab, newline), with the geometry the slash handling needs.
type cursorWord struct {
	text  string
	row   int
	start int
	end   int
	full  int
}

// cursorWordAt returns the word under the prompt's cursor, or an empty text
// when the cursor sits on a separator or outside the buffer. full is the
// word's start offset in the whole prompt, which is what lets callers tell a
// command typed at the very start of the line from a mid-sentence one.
func (m *Model) cursorWordAt() cursorWord {
	v := m.prompt.Value()
	lines := strings.Split(v, "\n")
	row := m.prompt.Line()
	res := cursorWord{text: "", row: row, start: 0, end: 0, full: 0}
	if row < 0 || row >= len(lines) {
		return res
	}
	line := lines[row]
	idx := m.prompt.Column() - 1
	if idx < 0 || idx >= len(line) || wordGap(line[idx]) {
		return res
	}
	start := idx
	for start > 0 && !wordGap(line[start-1]) {
		start--
	}
	end := idx
	for end < len(line) && !wordGap(line[end]) {
		end++
	}
	full := start
	for i := range row {
		full += len(lines[i]) + 1
	}
	return cursorWord{text: line[start:end], row: row, start: start, end: end, full: full}
}

// replaceCursorWord rebuilds the prompt with pick in place of the slash word
// the cursor sits on, plus a trailing space, preserving the text around it.
// When the cursor is not on a slash word it falls back to replacing the first
// /command, so text typed past a command still completes in place.
func (m *Model) replaceCursorWord(pick string) string {
	w := m.cursorWordAt()
	if !strings.HasPrefix(w.text, "/") {
		return menu.ReplaceCommand(m.prompt.Value(), pick)
	}
	lines := strings.Split(m.prompt.Value(), "\n")
	lines[w.row] = lines[w.row][:w.start] + pick + " " + lines[w.row][w.end:]
	return strings.Join(lines, "\n")
}

// wordGap reports whether r separates words, mirroring the delimiter set the
// menu uses to split a /command from the text around it: space, tab, newline.
func wordGap(r byte) bool {
	return r == 0x20 || r == 0x09 || r == 0x0a
}
