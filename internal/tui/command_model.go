package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/menu"
	"github.com/FacileStudio/bulle/internal/provider"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/nacelle"
)

func (m *Model) modelCmd(arg string) tea.Cmd {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		return m.openModelMenu()
	}
	return m.switchModel(trimmed)
}

func profileMenuItems() []menu.Item {
	dir, err := settings.ProfilesDir()
	if err != nil {
		return nil
	}
	profiles, err := settings.LoadProfiles(dir)
	if err != nil {
		return nil
	}
	var items []menu.Item
	for _, p := range profiles {
		items = append(items, menu.Item{
			Value:       "/model " + p.Name,
			Description: "profile · " + p.Provider.Backend + "/" + p.Provider.Model,
		})
	}
	return items
}

func (m *Model) openModelMenu() tea.Cmd {
	items := profileMenuItems()
	for _, cm := range provider.AvailableCatalogModels() {
		items = append(items, menu.Item{
			Value:       fmt.Sprintf("/model %s/%s", cm.Backend, cm.Model),
			Description: cm.Description,
		})
	}
	if len(items) == 0 {
		m.say(fromClient, "no profiles or models discovered")
		return nil
	}
	m.menu = *menu.New(items)
	m.menu.Filtered = items
	m.menu.Selected = 0
	m.menu.Dismissed = false
	m.modelPicker = true
	return nil
}

func findLoadedProfile(name string) (settings.Profile, bool) {
	dir, err := settings.ProfilesDir()
	if err != nil {
		return settings.Profile{}, false
	}
	profiles, err := settings.LoadProfiles(dir)
	if err != nil {
		return settings.Profile{}, false
	}
	return settings.FindProfile(profiles, name)
}

func (m *Model) switchModel(arg string) tea.Cmd {
	if m.run.busy {
		m.say(fromClient, "cannot switch model while a run is in progress")
		return nil
	}
	if p, ok := findLoadedProfile(arg); ok {
		return m.applyProfileSwitch(p)
	}
	return m.applyDirectModelSwitch(arg)
}

func (m *Model) applyProfileSwitch(p settings.Profile) tea.Cmd {
	backend, apiKey, err := profileBackend(p)
	if err != nil {
		m.say(fromClient, "failed to configure backend: "+err.Error())
		return nil
	}
	m.delegate.Thinking.Effort = nacelle.Effort(p.Reasoning.Effort)
	if p.Reasoning.Budget != nil {
		m.delegate.Thinking.Budget = *p.Reasoning.Budget
	} else {
		m.delegate.Thinking.Budget = 0
	}
	if p.Reasoning.Thinking != nil {
		m.delegate.Thinking.Show = *p.Reasoning.Thinking
	}
	m.delegate.Backend = nacelle.Retry(backend, nacelle.RetryOptions{})
	agent, err := nacelle.New(m.delegate)
	if err != nil {
		m.say(fromClient, "failed to create agent: "+err.Error())
		return nil
	}
	m.agent = agent
	m.activate(p.Provider.Backend, p.Provider.Model, p.Provider.BaseURL, apiKey)
	m.say(fromClient, fmt.Sprintf("switched to profile %s (%s/%s)", p.Name, p.Provider.Backend, p.Provider.Model))
	return nil
}

func (m *Model) applyDirectModelSwitch(target string) tea.Cmd {
	targetBackend, targetModel := parseTargetModel(target, m.activeBackend)
	var baseURL, apiKey string
	if targetBackend == m.activeBackend {
		baseURL = m.activeBaseURL
		apiKey = m.activeAPIKey
	}
	backend, err := provider.New(provider.Config{
		Backend: targetBackend,
		Model:   targetModel,
		BaseURL: baseURL,
		APIKey:  apiKey,
	})
	if err != nil {
		m.say(fromClient, "failed to switch model: "+err.Error())
		return nil
	}
	m.delegate.Backend = nacelle.Retry(backend, nacelle.RetryOptions{})
	agent, err := nacelle.New(m.delegate)
	if err != nil {
		m.say(fromClient, "failed to create agent: "+err.Error())
		return nil
	}
	m.agent = agent
	m.activate(targetBackend, targetModel, baseURL, apiKey)
	m.say(fromClient, fmt.Sprintf("switched to %s/%s", targetBackend, targetModel))
	return nil
}

func parseTargetModel(target, currentBackend string) (string, string) {
	prefix, rest, found := strings.Cut(target, "/")
	if found && (prefix == "anthropic" || prefix == "openai" || prefix == "google" || prefix == "openrouter") {
		return prefix, rest
	}
	return currentBackend, target
}
