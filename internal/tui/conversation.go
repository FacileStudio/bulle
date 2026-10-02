package tui

import (
	"encoding/json"
	"strings"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/bulle/internal/layout"
	"github.com/FacileStudio/bulle/internal/menu"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/skills"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/bulle/internal/tasks"
	"github.com/FacileStudio/bulle/internal/thinking"
	"github.com/FacileStudio/bulle/internal/toolview"
	"github.com/FacileStudio/nacelle"
)

type editChange = diff.EditChange
type skill = skills.Skill
type taskList = tasks.TaskList
type taskUpdate = tasks.TaskUpdate
type toolGroup = toolview.Group
type toolError = toolview.ToolError
type thoughts = thinking.Thoughts

var bySkillName = skills.BySkillName
var skillCommandNames = skills.SkillCommandNames
var skillPrompt = skills.SkillPrompt

type approvalDecision = approval.Decision
type approvalRequest = approval.Request

// Approvals aliases approval.Approvals.
type Approvals = approval.Approvals

// ApprovalRequest aliases approval.Request.
type ApprovalRequest = approval.Request

const (
	denied            = approval.Denied
	allowedOnce       = approval.AllowedOnce
	allowedForSession = approval.AllowedForSession
)

// Config aliases settings.Config.
type Config = settings.Config

var listSessionFiles = sessions.ListSessionFiles
var loadSession = sessions.LoadSession
var formatSessionEntry = sessions.FormatSessionEntry
var restoreAtLaunch = sessions.RestoreAtLaunch

var priorContents = diff.PriorContents
var renderDiff = diff.RenderDiff
var captureEdit = diff.CaptureEdit

var truncate = layout.Truncate
var unstyled = layout.Unstyled

var shortTokens = status.ShortTokens
var waitingVerb = status.WaitingVerb
var lasted = status.Lasted
var took = status.Took

const statusDone = "completed"

// BuildApprovals constructs the approval gate and returns the approval function.
func BuildApprovals(config Config) (*Approvals, nacelle.Approve) {
	return approval.Build(*config.ApproveTools)
}

func (m *Model) record(event nacelle.Event) {
	switch event.Kind {
	case nacelle.KindText:
		m.closeResults()
	case nacelle.KindToolCall:
		m.closeResults()
		m.run.asked = append(m.run.asked, nacelle.ToolCall{
			ID:       event.Tool.ID,
			Name:     event.Tool.Name,
			Input:    json.RawMessage(event.Tool.Input),
			Finished: true,
		})
	case nacelle.KindToolResult:
		if event.Tool.Discarded {
			m.forgetAsked(event.Tool.ID)
			return
		}
		m.closeTurn("")
		m.run.answered = append(m.run.answered, nacelle.ToolResult{
			ID:     event.Tool.ID,
			Name:   event.Tool.Name,
			Result: event.Tool.Result,
			Failed: event.Tool.Err != nil,
		})
	}
}

func (m *Model) forgetAsked(id string) {
	kept := m.run.asked[:0]
	for _, call := range m.run.asked {
		if part, ok := call.(nacelle.ToolCall); !ok || part.ID != id {
			kept = append(kept, call)
		}
	}
	m.run.asked = kept
}

func (m *Model) closeResults() {
	if len(m.run.answered) == 0 {
		return
	}
	m.conversation = append(m.conversation, nacelle.Message{Role: nacelle.RoleUser, Parts: m.run.answered})
	m.run.answered = nil
}

func (m *Model) closeTurn(stop nacelle.Stop) {
	parts := m.run.asked
	m.run.asked = nil

	if said := m.flush(); said != "" {
		parts = append([]nacelle.Part{nacelle.Text{Text: said}}, parts...)
	}
	if stop != "" {
		parts = append(parts, nacelle.Finish{Stop: stop})
	}
	if len(parts) == 0 {
		return
	}
	m.conversation = append(m.conversation, nacelle.Message{Role: nacelle.RoleAssistant, Parts: parts})
}

func (m *Model) dropUnanswered() { m.run.asked = nil }

// editing is the queued line being actively rewritten, or -1 when none is.
//
// The marker (hist.FromEnd) is only meaningful while the prompt actually holds
// a draft: deliver skips the edited line so a settle does not send half a
// rewrite, but that skip must be released the moment the user leaves the
// prompt. Walking through the queue with up/down leaves FromEnd pointing at a
// line long after the user stopped editing it — with no prompt it is stale,
// so it is cleared here and the line is free to go out with its turn.
func (m *Model) editing() int {
	i := m.hist.Editing(m.Len())
	if i < 0 || m.prompt.Value() != "" {
		return i
	}
	m.hist.FromEnd = 0
	return -1
}

func menuItems(skills map[string]skill) []menu.Item {
	names := commandNames()
	skillNames := skillCommandNames(skills)
	items := make([]menu.Item, 0, len(names)+len(skillNames))
	for _, name := range names {
		items = append(items, menu.Item{Value: name})
	}
	for _, name := range skillNames {
		items = append(items, menu.Item{
			Value:       name,
			Description: skills[strings.TrimPrefix(name, "/skill:")].Description,
		})
	}
	return items
}
