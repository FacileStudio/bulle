package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/herdr"
	"github.com/FacileStudio/bulle/internal/menu"
)

// key handles this client's bindings, reporting whether it consumed the
// press. Anything the scroller does not claim belongs to the prompt.
//
// Ctrl+C cancels a run in flight and only quits when nothing is running, so
// a long answer can be abandoned without losing the session — the terminal
// is in raw mode, so nothing quits on Ctrl+C unless this says so. That needs
// an escape hatch: busy only clears once settle sees the results channel
// close, and a tool wedged on a subprocess never closes it. A second ctrl+c
// inside forceQuit, or ctrl+\ at any time, quits regardless — otherwise the
// only way out of an alt-screen raw-mode terminal is kill -9 from elsewhere.
//
// Both stay live while a tool approval is pending too: a question nobody
// answers must not be a second way to get stuck. See decide's doc comment
// for why cancelling clears run.pending directly instead.
//
// The dropdown menu is checked next, ahead of both enter and the scroller:
// while it's open, up/down/tab/enter/esc belong to picking a command, not
// to scrolling the transcript or sending what's typed. Anything the menu
// itself does not claim (an ordinary character, backspace) falls all the
// way through to the prompt, which is what keeps its own filter editable.
//
// Ctrl+t sits above esc and takes the press unconditionally — see reveal for
// what it does with nothing to expand, and why that is not the call escaped
// makes. Esc reports an idle press unhandled because esc is the key every
// terminal reader uses to back out of something; nobody presses ctrl+t at a
// prompt meaning anything at all.
//
// Nobody except the textarea, which binds it to transpose-character-backward
// and now never sees it. That is the trade taken knowingly: transposing the
// two characters behind the cursor is an emacs habit almost nothing in this
// prompt is edited by, and reading the model's reasoning is a thing somebody
// wants several times a session.
//
// Esc stops a run and does nothing else, which is the whole reason it is
// worth having next to a ctrl+c that already cancels: the key that stops the
// answer is then never the key that might close the client, so there is no
// press that has to be thought about first. It sits below the menu on
// purpose — esc closes the dropdown before it stops anything, because a
// dropdown standing open is the nearer thing to back out of, and it is what
// esc already meant there. See escaped for what it does with no run to stop.
func (m *Model) controlKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "ctrl+\\":
		return true, tea.Quit
	case "ctrl+d":
		if m.run.busy || m.prompt.Value() == "" {
			return true, m.detachCmd()
		}
	case "ctrl+c":
		if !m.run.busy || time.Since(m.run.interrupted) < forceQuit {
			return true, tea.Quit
		}
		m.abandon()
		return true, nil
	}
	return false, nil
}

func (m *Model) key(press tea.KeyPressMsg) (bool, tea.Cmd) {
	if handled, cmd := m.controlKey(press); handled {
		return true, cmd
	}
	if m.run.pending != nil {
		m.decide(press)
		return true, nil
	}
	if m.menu.Open() && m.navigateMenu(press) {
		return true, nil
	}
	return m.promptKey(press)
}

// promptKey handles the keys that act on the prompt's own text, passed through
// once nothing above claims the press. Anything it does not bind falls through
// to history navigation.
func (m *Model) promptKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	keyStr := press.String()
	if m.run.promptEditKey != "" && (keyStr == m.run.promptEditKey || strings.EqualFold(keyStr, m.run.promptEditKey)) {
		return true, m.openEditor()
	}
	switch keyStr {
	case "ctrl+t":
		return m.reveal()
	case "ctrl+g":
		return true, m.openEditor()
	case "esc":
		return m.escaped()
	case "tab":
		return m.tabKey(), nil
	case "alt+enter", "shift+enter":
		return false, nil
	case "enter":
		return true, m.ask()
	}
	return m.historyKey(press)
}

func (m *Model) tabKey() bool {
	cmd := m.commandFilter()
	if cmd == "" {
		return false
	}
	matches := menu.FilterMenu(m.menu.Items, cmd)
	m.menu.Reset()
	if len(matches) == 1 {
		m.prompt.SetValue(m.replaceCursorWord(matches[0].Value))
		m.prompt.CursorEnd()
		return true
	}
	if len(matches) > 1 {
		m.menu.Filtered = matches
		m.layout(m.windowHeight)
		return true
	}
	return false
}

func (m *Model) decide(press tea.KeyPressMsg) {
	var decision approval.Decision
	switch press.String() {
	case "y":
		decision = approval.AllowedOnce
	case "a":
		decision = approval.AllowedForSession
	case "n":
		decision = approval.Denied
	default:
		return
	}

	pending := m.run.pending
	m.run.pending = nil
	pending.Decision <- decision
	herdr.Report(m.herdrClient, herdr.Working)
}

func (m *Model) refreshMenu() {
	m.prompt.SetStyles(m.promptStyles)
	if m.modelPicker {
		m.modelPicker = false
		m.menu = *menu.New(menuItems(m.skills))
	}
	w := m.cursorWordAt()
	if strings.HasPrefix(w.text, "/") && w.full == 0 {
		m.menu.Filter(w.text)
	} else {
		m.menu.Reset()
	}
	m.layout(m.windowHeight)
}

func (m *Model) navigateMenu(press tea.KeyPressMsg) bool {
	switch press.String() {
	case "up", "ctrl+k":
		m.menu.Up()
	case "down", "ctrl+j":
		m.menu.Down()
	case "tab", "enter":
		if it, ok := m.menu.SelectedItem(); ok {
			if m.modelPicker {
				m.modelPicker = false
				m.menu.Dismiss()
				m.menu = *menu.New(menuItems(m.skills))
				m.switchModel(strings.TrimPrefix(it.Value, "/model "))
			} else {
				m.prompt.SetValue(m.replaceCursorWord(it.Value))
				m.prompt.CursorEnd()
				m.menu.Dismiss()
			}
		}
	case "esc":
		if m.modelPicker {
			m.modelPicker = false
			m.menu = *menu.New(menuItems(m.skills))
		}
		m.menu.Dismiss()
	default:
		return false
	}
	m.menu.ClampView()
	m.layout(m.windowHeight)
	return true
}

func (m *Model) historyKey(press tea.KeyPressMsg) (bool, tea.Cmd) {
	switch press.String() {
	case "up":
		if pos := m.prompt.Cursor(); pos != nil && pos.Y > 0 {
			return false, nil
		}
		if text, ok := m.hist.Recall(m.prompt.Value(), m.Items()); ok {
			m.prompt.Reset()
			m.prompt.SetValue(text)
			m.prompt.MoveToEnd()
			return true, nil
		}
	case "down":
		if text, ok := m.hist.Advance(m.Items()); ok {
			m.prompt.Reset()
			m.prompt.SetValue(text)
			m.prompt.MoveToEnd()
			return true, nil
		}
	}
	return false, nil
}
