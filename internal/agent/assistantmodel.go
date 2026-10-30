package agent

import (
	"context"
	"io"

	"github.com/FacileStudio/bulle/internal/settings"
)

// AssistantProfile returns the profile the assistant model calls use: the
// given name, else the chat profile from the config.
func AssistantProfile(name string) (string, error) {
	if name != "" {
		return name, nil
	}
	cfg, err := loadCronConfig()
	if err != nil {
		return "", err
	}
	return cfg.Chat.Profile, nil
}

// RunHeadlessPrompt runs one prompt through the headless agent with the named
// profile, tools off, and returns the text. Cron jobs resolve config the same way.
func RunHeadlessPrompt(ctx context.Context, profile, prompt string) (string, error) {
	ensureUserPath()
	cfg, err := loadCronConfig()
	if err != nil {
		return "", err
	}
	if profile != "" {
		if err := settings.ResolveProfile(&cfg, profile); err != nil {
			return "", err
		}
	}
	off := false
	cfg.Bash, cfg.ApproveTools = &off, &off
	text, _, err := runHeadlessConfigToContext(ctx, io.Discard, prompt, cfg, nil)
	return text, err
}

// AssistantSettings returns the assistant block of the resolved config.
func AssistantSettings() (settings.Assistant, error) {
	cfg, err := loadCronConfig()
	return cfg.Assistant, err
}
