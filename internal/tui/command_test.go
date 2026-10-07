package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/menu"
	"github.com/FacileStudio/bulle/internal/skills"
	"github.com/FacileStudio/nacelle"
)

func TestMenuItemsListsCommandsBeforeSkillsWithDescriptions(t *testing.T) {
	items := menuItems(map[string]skills.Skill{"deploy": {Name: "deploy", Description: "ships the app"}})

	if len(items) != len(commands)+1 {
		t.Fatalf("menuItems = %+v, want every command plus the one skill", items)
	}
	last := items[len(items)-1]
	if last.Value != "/skill:deploy" || last.Description != "ships the app" {
		t.Errorf("last item = %+v, want the skill, with its own description", last)
	}
	for _, it := range items[:len(items)-1] {
		if it.Description != "" {
			t.Errorf("command %q carried a description %q, want none", it.Value, it.Description)
		}
	}
}

// TestMenuItemsListsCommandsBeforeSkillsWithDescriptionsShort tests that menuItems
// returns the correct number of items when the skills map is empty.
func TestMenuItemsListsCommandsBeforeSkillsWithDescriptionsShort(t *testing.T) {
	items := menuItems(map[string]skills.Skill{})

	if len(items) != len(commands) {
		t.Fatalf("menuItems = %+v, want only commands", items)
	}
	for _, it := range items {
		if it.Description != "" {
			t.Errorf("command %q carried a description %q, want none", it.Value, it.Description)
		}
	}
}

func TestRefreshMenuFiltering(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	if !m.menu.Open() || len(m.menu.Filtered) != len(commands) {
		t.Errorf("expected open with all commands on '/'")
	}

	m.prompt.SetValue("/cl")
	m.refreshMenu()
	if len(m.menu.Filtered) != 1 || m.menu.Filtered[0].Value != "/clear" {
		t.Errorf("filtered = %+v, want /clear", m.menu.Filtered)
	}

	m.prompt.SetValue("/clear")
	m.refreshMenu()
	if m.menu.Open() {
		t.Error("menu stayed open on fully typed command")
	}

	m.menu.Items = []menu.Item{{Value: "/skill:review"}, {Value: "/skill:review-pr"}}
	m.prompt.SetValue("/skill:review-")
	m.refreshMenu()
	if !m.menu.Open() {
		t.Error("menu did not reopen when typing past exact match")
	}

	m.prompt.SetValue("hello")
	m.refreshMenu()
	if m.menu.Open() {
		t.Error("menu stayed open when line no longer started with slash")
	}
}

func TestKeyEnterSendsAFullyTypedCommandInsteadOfRepickingIt(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/clear")
	m.refreshMenu()

	m.key(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := m.prompt.Value(); got != "" {
		t.Errorf("prompt = %q after enter, want empty", got)
	}
}

func TestTabOnMidSentenceSlashPreservesTextBeforeAndAfter(t *testing.T) {
	m := sized()
	m.prompt.SetValue("please run /cl now")

	m.refreshMenu()
	if m.menu.Open() {
		t.Fatal("mid-sentence slash command auto-opened the menu")
	}

	m.key(tea.KeyPressMsg{Code: tea.KeyTab})
	if got, want := m.prompt.Value(), "please run /clear  now"; got != want {
		t.Errorf("tab did not preserve surrounding text: prompt = %q, want %q", got, want)
	}
}

func TestNavigateMenuSelectionTabAndEsc(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.menu.Selected != 0 {
		t.Errorf("selected = %d after up at top", m.menu.Selected)
	}

	for range m.menu.Filtered {
		m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.menu.Selected != len(m.menu.Filtered)-1 {
		t.Errorf("selected = %d after down to end", m.menu.Selected)
	}

	m.prompt.SetValue("/cl")
	m.refreshMenu()
	handled := m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyTab})
	if !handled || m.prompt.Value() != "/clear " || m.run.busy || m.menu.Open() {
		t.Errorf("tab failed: handled=%v prompt=%q busy=%v open=%v", handled, m.prompt.Value(), m.run.busy, m.menu.Open())
	}

	m.prompt.SetValue("/cl")
	m.refreshMenu()
	handled = m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || m.prompt.Value() != "/cl" || m.menu.Open() {
		t.Errorf("esc failed: handled=%v prompt=%q open=%v", handled, m.prompt.Value(), m.menu.Open())
	}
}

func TestViewMenuRendering(t *testing.T) {
	m := sized()
	if got := m.viewMenu(); got != "" {
		t.Errorf("viewMenu() = %q, want empty when closed", got)
	}

	m.prompt.SetValue("/")
	m.refreshMenu()

	first, _, _ := strings.Cut(visible(m.viewMenu()), "\n")
	if !strings.HasPrefix(first, "→ /background") {
		t.Errorf("first row = %q, want selected marker", first)
	}

	m.navigateMenu(tea.KeyPressMsg{Code: tea.KeyDown})
	lines := strings.Split(visible(m.viewMenu()), "\n")
	if !strings.HasPrefix(lines[0], "  /background") || !strings.HasPrefix(lines[1], "→ /bg") {
		t.Errorf("selection move failed: lines[0]=%q lines[1]=%q", lines[0], lines[1])
	}

	got := m.viewMenu()
	for _, want := range []string{"/background", "/bg", "/clear", "/compact"} {
		if !strings.Contains(got, want) {
			t.Errorf("viewMenu() missing %q", want)
		}
	}
}

func TestKeyRoutesUpDownToTheMenuInsteadOfScrollingWhileItIsOpen(t *testing.T) {
	m := sized()
	m.prompt.SetValue("/")
	m.refreshMenu()
	handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyDown})

	if m.menu.Selected == 0 || !handled {
		t.Error("down was not handled by open menu")
	}
}

func TestSecondSlashTypedMidSentenceCompletesWhenTabbed(t *testing.T) {
	m := sized()
	m.menu.Items = []menu.Item{{Value: "/skill:facile-review"}, {Value: "/skill:muse"}}
	m.prompt.SetValue("/skill:facile-review /mus")

	m.refreshMenu()
	if m.menu.Open() {
		t.Fatal("menu auto-opened on a mid-sentence slash, want it closed until tab")
	}

	handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyTab})
	if !handled || m.prompt.Value() != "/skill:facile-review /skill:muse " {
		t.Errorf("tab: handled=%v prompt=%q, want the second slash completed", handled, m.prompt.Value())
	}
}

func TestTabOpensTheMenuForAnAmbiguousMidSentenceSlash(t *testing.T) {
	m := sized()
	m.menu.Items = []menu.Item{{Value: "/skill:facile-review"}, {Value: "/skill:facile-plan"}, {Value: "/skill:muse"}}
	m.prompt.SetValue("/skill:facile-review /f")

	m.refreshMenu()
	if m.menu.Open() {
		t.Fatal("menu auto-opened on a mid-sentence slash, want it closed until tab")
	}

	handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyTab})
	if !handled || !m.menu.Open() || len(m.menu.Filtered) != 2 {
		t.Errorf("tab: handled=%v open=%v filtered=%+v, want the menu open on the two /f matches",
			handled, m.menu.Open(), m.menu.Filtered)
	}
}

func TestSlashCostReportsTheSessionTotal(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 400, OutputTokens: 100, Cost: 0.01}
	m.run.usage = nacelle.Usage{InputTokens: 600, OutputTokens: 900, CacheReadTokens: 2000, Cost: 0.02}
	m.tools, m.failed = 3, 1
	m.began = time.Now().Add(-90 * time.Second)

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	for _, want := range []string{"in 1.0k", "out 1.0k", "2.0k cached", "$0.0300", "3 tools · 1 failed", "session · 1m30s"} {
		if !strings.Contains(said, want) {
			t.Errorf("said = %q, want %q", said, want)
		}
	}
}

func TestSlashCostOnAnEmptySession(t *testing.T) {
	m := sized()

	m.prompt.SetValue("/cost")
	said := printedBy(m.ask())
	if strings.Contains(said, "$") || strings.Contains(said, "tool") {
		t.Errorf("said = %q, want no cost and no tools for empty session", said)
	}
	if !strings.Contains(said, "session · ") {
		t.Errorf("said = %q, want session line", said)
	}
}
