package tui

import (
	"encoding/json"
	"strings"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/menu"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/skills"
	"github.com/FacileStudio/nacelle"
)

const statusDone = "completed"

// BuildApprovals constructs the approval gate and returns the approval function.
func BuildApprovals(config settings.Config) (*approval.Approvals, nacelle.Approve) {
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

func (m *Model) editing() int {
	i := m.hist.Editing(m.Len())
	if i < 0 || m.prompt.Value() != "" {
		return i
	}
	m.hist.FromEnd = 0
	return -1
}

func menuItems(sks map[string]skills.Skill) []menu.Item {
	names := commandNames()
	skillNames := skills.SkillCommandNames(sks)
	items := make([]menu.Item, 0, len(names)+len(skillNames))
	for _, name := range names {
		items = append(items, menu.Item{Value: name})
	}
	for _, name := range skillNames {
		items = append(items, menu.Item{
			Value:       name,
			Description: sks[strings.TrimPrefix(name, "/skill:")].Description,
		})
	}
	return items
}
