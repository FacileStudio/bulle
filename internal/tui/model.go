// Package tui implements the Bubble Tea terminal user interface.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/bulle/internal/herdr"
	"github.com/FacileStudio/bulle/internal/history"
	"github.com/FacileStudio/bulle/internal/menu"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/skills"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/bulle/internal/theme"
	"github.com/FacileStudio/nacelle"
)

// forceQuit is how long the offer to quit outright stays open after a run is
// first asked to stop — long enough to read the status line, short enough that
// a ctrl+c minutes later still means "stop this run", not "quit".
const forceQuit = 3 * time.Second

func initialInflight(c SessionConfig) inflight {
	return inflight{
		runControl:    runControl{cancel: func() {}},
		promptEditKey: c.Editor.PromptEditKey,
		editState: editState{
			edits:      map[string]diff.EditChange{},
			editorPath: c.Editor.Editor,
		},
	}
}

// NewModel builds the client. The banner names the backend and model, so
// which provider is billed is visible before typing, not after it fails.
// skills is every skill loaded this run — kept keyed by name so
// /skill:name is a lookup, not a scan, every time it's typed, and listed
// alongside the client's own commands in the dropdown menu. startMessage, when
// non-empty, is printed as the first thing on screen — a welcome block or an
// ascii banner — above the banner itself, each separated by a blank row.
func NewModel(agent *nacelle.Agent, banner string, sks []skills.Skill, c SessionConfig) *Model {
	byName := skills.BySkillName(sks)
	base := theme.Themed(true)

	policy := resolvedPolicy(c.Compaction.Policy)
	engine := compaction.NewEngine(policy, c.Compaction.Judge, nil)
	m := &Model{
		core:          core{agent: agent, banner: banner, autoResume: c.AutoResume, resumePath: c.Resume, herdrClient: herdr.NewFromEnv()},
		transcript:    transcript{compactAt: policy.Ceiling, compactor: engine},
		composer:      composer{prompt: newPrompt(c.PromptPlaceholder, base.Border), hist: history.New()},
		look:          look{theme: base, spin: status.NewSpinner(), showHooks: c.Hooks.Show, showHookOutput: c.Hooks.ShowOutput},
		account:       account{began: time.Now(), grind: grindBudget{cost: c.Grind.Cost, tokens: c.Grind.Tokens, cap: c.Grind.Continuations}},
		screen:        screen{width: 80, liveRows: 1},
		commandState:  commandState{skills: byName, menu: *menu.New(menuItems(byName))},
		parallelState: parallelState{maxConcurrency: c.MaxConcurrency},
		run:           initialInflight(c),
	}
	engine.Builder = func() (*nacelle.Agent, error) {
		if m.agent == nil {
			return nil, nil
		}
		return compaction.BackendAgent(m.agent.Backend())()
	}
	m.pretty = theme.Prettier(m.theme.Markdown, max(m.width-2, 1))
	m.promptStyles = m.prompt.Styles()
	if c.StartMessage != "" {
		m.say(fromStart, strings.TrimRight(c.StartMessage, "\n")+"\n")
	}
	m.say(fromClient, banner+"\n")
	if c.Startup.SystemTokens > 0 {
		m.say(fromClient, startupContextNote(c.Startup))
		m.say(fromClient, fmt.Sprintf("system prompt loaded · ~%s tokens", status.ShortTokens(c.Startup.SystemTokens)))
	}
	herdr.Report(m.herdrClient, herdr.Idle)
	return m
}

// Init asks the terminal what colour it is, and opens the watches that
// carry work in from goroutines this loop does not own — a delegated run's
// spend, the plan the task tool reports, the titles the task-row summarizer
// returns, and the detached subagent results a /parallel fan-out posts.
//
// All of them have to be armed here; a watcher only re-armed by its own
// message never sees a first run, so the feature silently does nothing.
//
// There is no cursor-blink command because the prompt draws no cursor of its
// own — see newPrompt's SetVirtualCursor(false); the caret on screen is the
// terminal's, positioned by View, and a blink tick for a cursor nobody renders
// is a timer that wakes the program up to change nothing.
func (m *Model) Init() tea.Cmd {
	if res := sessions.RestoreAtLaunch(m.resumePath, m.run.root, m.autoResume); res.Conversation != nil {
		m.conversation = res.Conversation
		savedSession := m.session
		m.session = nil
		for _, msg := range res.Conversation {
			text := messageText(msg)
			switch msg.Role {
			case nacelle.RoleUser:
				m.say(fromReader, text)
			case nacelle.RoleAssistant:
				m.say(fromModel, text)
			}
		}
		m.session = savedSession
		m.say(fromClient, fmt.Sprintf("resumed session %s (%d messages)", res.Name, len(res.Conversation)))
	} else if res.Error != "" {
		m.say(fromClient, res.Error)
	}
	return tea.Batch(tea.RequestBackgroundColor, watchDelegations(), watchTasks(), watchTitles(), watchDetached(), watchUpdates(), watchHooks(), m.startupDiagnostics())
}

// Update routes each message to the one place that owns it, and hands whatever
// that had to say to the terminal before whatever it started.
//
// That ordering is the whole of a bug this client shipped on two paths at
// once. Two of the commands route returns block until the model sends
// something — waitFor, and the batch send wraps it in — and a sequence does
// not reach its next command until the one before it is done, so a line said
// on the way into a wait was not drawn until that wait ended. The question
// appeared when the answer did rather than when it was asked, and a tool's
// call line waited on the tool it announced: six seconds late for a
// six-second command, measured. Nothing already said is worth less than the
// thing it is waiting for. Fixing it here rather than in those two commands
// is the point — a third would arrive with no reason to know it had to flush
// first, which is how the second sat there while the first was being fixed.
//
// Two statements, not one call: as arguments they are evaluated left to right,
// which would drain the queue before the message that fills it was routed.
//
// Sequence, not Batch: a batch makes no promise about the order its commands
// run in, and the routed cmd may be tea.Quit — a quit that wins that race
// takes the last thing said with it, which for a queued /quit is the echo of
// the line that quit.
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	started := m.route(message)
	return m, tea.Sequence(m.prints(), started)
}

// route is Update's own body, split out so draining the print queue is one seam.
// It is a tagged-union dispatcher: every message type hands off to its one
// owner and returns. The only arms that carry statements are the ones with
// real logic — a key the command palette did not consume, a palette switch,
// and the compaction-done branch; the rest are one-line forwards, and anything
// the switch does not match falls through to promptRoute. The frame is long
// because the dispatch is wide (13 arms plus the catch-all), not because any
// arm does much.
func (m *Model) route(message tea.Msg) tea.Cmd {
	if cmd, ok := m.routeTask(message); ok {
		return cmd
	}
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		return m.resize(message)
	case tea.KeyPressMsg:
		return m.keyOrPrompt(message)
	case tea.BackgroundColorMsg:
		return m.retheme(message)
	case spinner.TickMsg:
		return m.spun(message)
	case approval.Request:
		return m.parkApproval(message)
	case result:
		return m.consume(message)
	case finished:
		return m.settle()
	case compaction.Outcome:
		return m.settleCompaction(message)
	case compactFinished:
		return m.finishCompaction()
	case startupDiagnostics:
		return m.recordStartupDiagnostics(message)
	case editorFinishedMsg:
		return m.finishEditor(message)
	case tea.KeyboardEnhancementsMsg:
		return nil
	}

	return m.promptRoute(message)
}

// keyOrPrompt routes a handled key to the command palette, and sends one that
// was not consumed there on to the prompt — the route fall-through, folded into
// a call so the dispatcher stays one line per arm.
func (m *Model) keyOrPrompt(press tea.KeyPressMsg) tea.Cmd {
	if handled, cmd := m.key(press); handled {
		return cmd
	}
	return m.promptRoute(press)
}

// finishCompaction closes out a finished pass: the run that was holding the
// channel collapses, and the pending run resumes now that the context is free.
// Nothing queued means nothing waited on the pass, so there is no run to start.
// On the idle path — no run waiting — the lines typed while the pass ran are
// delivered here, exactly as settleCompaction does with an outcome, so a pass
// that ended without one never strands them.
func (m *Model) finishCompaction() tea.Cmd {
	m.compacting = false
	m.run.compactChan = nil
	if m.run.busy && m.agent != nil {
		return m.startRun(m.run.bgCtx)
	}
	return m.deliver()
}

// promptRoute forwards unhandled messages to the prompt and refreshes the dropdown.
// A wheel is the one thing never handed over: it is exclusive to scrolling the
// tui transcript, so it can never read as up/down and drag the input's recall
// history into view.
func (m *Model) promptRoute(message tea.Msg) tea.Cmd {
	if msg, ok := message.(tea.PasteMsg); ok {
		return m.insertPaste(msg.Content)
	}
	if msg, ok := message.(tea.MouseWheelMsg); ok {
		return m.scrollWheel(msg)
	}
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(message)
	m.refreshMenu()
	return cmd
}
