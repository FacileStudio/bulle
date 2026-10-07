package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/cost"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/skills"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/bulle/internal/tasks"
	"github.com/FacileStudio/nacelle"
)

type command func(m *Model) tea.Cmd

var commands = map[string]command{
	"background": (*Model).detachCmd,
	"bg":         (*Model).detachCmd,
	"clear":      (*Model).clear,
	"compact":    (*Model).compactCmd,
	"cost": func(m *Model) tea.Cmd {
		m.say(fromClient, cost.Summary(m.total(), m.tools, m.failed, time.Since(m.began)))
		return nil
	},
	"detach":   (*Model).detachCmd,
	"help":     (*Model).help,
	"model":    func(m *Model) tea.Cmd { return m.modelCmd("") },
	"quit":     func(_ *Model) tea.Cmd { return tea.Quit },
	"resume":   (*Model).resumeCmd,
	"sessions": (*Model).sessionsCmd,
	"status":   (*Model).statusCmd,
}

func (m *Model) parseCommand(line string) (command, bool) {
	if !strings.HasPrefix(line, "/") {
		return nil, false
	}
	name, rest, _ := strings.Cut(line[1:], " ")
	if name == "parallel" {
		return func(m *Model) tea.Cmd { return m.handleParallelCommand(rest) }, true
	}
	if name == "model" {
		return func(m *Model) tea.Cmd { return m.modelCmd(rest) }, true
	}
	if cmd, ok := commands[name]; ok {
		return cmd, true
	}
	if skillName, ok := strings.CutPrefix(name, "skill:"); ok {
		if s, ok := m.skills[skillName]; ok {
			return runSkill(s, rest), true
		}
		return func(m *Model) tea.Cmd {
			m.say(fromClient, "unknown skill "+skillName+" — try /help")
			return nil
		}, true
	}
	return func(m *Model) tea.Cmd {
		m.say(fromClient, "unknown command "+line+" — try /help")
		return nil
	}, true
}

func commandNames() []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, "/"+name)
	}
	sort.Strings(names)
	return names
}

func (m *Model) clear() tea.Cmd {
	m.conversation = nil
	m.spent = nacelle.Usage{}
	m.rate = 0
	m.size, m.trimmed = 0, 0
	m.engine().ResetThrash()
	m.last = compacted{}
	m.tasks = nil
	tasks.SetCurrentPlan(nil)
	m.clearFinishedParallel()
	m.layout(m.windowHeight)
	m.Forget()
	m.held = nil
	echoed := m.prints()
	m.say(fromClient, m.banner+" · cleared")
	scrolled := strings.Repeat("\n", max(m.windowHeight, 1))
	return tea.Sequence(echoed, m.printed(scrolled), m.prints())
}

func (m *Model) help() tea.Cmd {
	m.say(fromClient, strings.Join([]string{
		"/clear — start a new session, same client",
		"/compact — summarize older turns now to free context (auto-compacts on its own over the threshold)",
		"/cost — what this session has spent so far",
		"/detach — detach session to background (also /bg, /background)",
		"/help — show this message",
		"/model [name] — switch active model or profile mid-session, or open model picker",
		"/quit — quit",
		"/resume — resume the most recent session for this project",
		"/sessions — list all available sessions for this project",
		"/status — session summary: questions, answers, tools, cached tokens, context size, elapsed time, log size",
		"/skill:name [what to do] — run a loaded skill directly, instead of waiting for the model to decide to",
		"/parallel — delegate multiple independent tasks to run concurrently",
		"/parallel cancel [batch] — stop a live parallel fan-out (or every one, with no batch)",
		"",
		"Esc stops a run and nothing else. Ctrl+C stops one too, or quits when idle; ctrl+\\ force-quits.",
		"Ctrl+D detaches the session to run in the background.",
		"Ctrl+T expands the reasoning collapsed to a single line, and keeps showing it in full until pressed again.",
		"Ctrl+G opens the external editor to edit the prompt.",
		"Enter during a run queues the line and sends it once the run finishes; stopping the run drops whatever is queued.",
		"The prompt wraps and grows as you type. Alt+Enter inserts a new line; Enter always sends.",
		"Scroll, select and copy with the terminal as usual — what was said is ordinary terminal output, not a window this client owns.",
		"Typing / opens a dropdown of commands and skills — up/down move, tab/enter pick, esc closes it.",
	}, "\n"))
	return nil
}

func (m *Model) statusCmd() tea.Cmd {
	var lines []string
	lines = append(lines, fmt.Sprintf("session · %s", status.Lasted(time.Since(m.began))))
	lines = append(lines, fmt.Sprintf("model · %s/%s", m.activeBackend, m.activeModel))
	lines = append(lines, fmt.Sprintf("tools · %d total · %d failed", m.tools, m.failed))
	total := m.total()
	lines = append(lines, "tokens · "+tokenTotals(total))
	if total.Cost > 0 {
		lines = append(lines, fmt.Sprintf("cost · %s", cost.FormatCost(total.Cost)))
	}
	if m.session != nil {
		if info, err := os.Stat(m.session.Path()); err == nil {
			lines = append(lines, fmt.Sprintf("log · %s (%d bytes)", m.session.Path(), info.Size()))
		}
		if m.session.HasWriteError() {
			lines = append(lines, "log · [!] write errors detected")
		}
	}
	if total.CacheReadTokens > 0 {
		lines = append(lines, fmt.Sprintf("cached · %s", status.ShortTokens(total.CacheReadTokens)))
	}
	lines = append(lines, m.compactionLines()...)
	if m.trimmed > 0 {
		lines = append(lines, fmt.Sprintf("⎇ · %d", m.trimmed))
	}
	if m.thrashed() {
		lines = append(lines, "thrash · context still over the threshold after a pass — /clear or read in chunks")
	}
	m.say(fromClient, strings.Join(lines, "\n"))
	return nil
}

func (m *Model) resumeCmd() tea.Cmd {
	projectRoot := m.run.root
	if projectRoot == "" {
		projectRoot = "."
	}
	sessionFiles := sessions.ListSessionFiles(projectRoot)
	if len(sessionFiles) == 0 {
		m.say(fromClient, "no previous sessions found for this project")
		return nil
	}
	mostRecent := sessionFiles[0]
	conversation := sessions.LoadSession(mostRecent)
	if conversation == nil {
		m.say(fromClient, "failed to load session: "+mostRecent)
		return nil
	}
	m.conversation = conversation
	m.say(fromClient, fmt.Sprintf("resumed session from %s (%d messages)",
		filepath.Base(mostRecent), len(conversation)))
	return nil
}

func (m *Model) sessionsCmd() tea.Cmd {
	projectRoot := m.run.root
	if projectRoot == "" {
		projectRoot = "."
	}
	sessionFiles := sessions.ListSessionFiles(projectRoot)
	if len(sessionFiles) == 0 {
		m.say(fromClient, "no previous sessions found for this project")
		return nil
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("sessions for project %s:", projectRoot))
	for _, filePath := range sessionFiles {
		lines = append(lines, sessions.FormatSessionEntry(filePath))
	}
	m.say(fromClient, strings.Join(lines, "\n"))
	return nil
}

func runSkill(s skills.Skill, args string) command {
	return func(m *Model) tea.Cmd {
		text, err := skills.SkillPrompt(s, args)
		if err != nil {
			m.say(fromClient, "reading "+s.Path+": "+err.Error())
			return nil
		}
		return m.send(text)
	}
}
