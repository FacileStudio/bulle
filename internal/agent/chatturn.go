package agent

import (
	"sync"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
)

func resolveChatConfig() (settings.Config, error) {
	ensureUserPath()
	flags := settings.FromFlags(settings.Defaults(""))
	cfg, err := settings.Settings(DefaultSystemPrompt(), flags)
	if err != nil {
		return settings.Config{}, err
	}
	if cfg.Chat.Profile != "" {
		if err := settings.ResolveProfile(&cfg, cfg.Chat.Profile); err != nil {
			return settings.Config{}, err
		}
	}
	return cfg, nil
}

var chatConfigOnce = sync.OnceValues(resolveChatConfig)

// ChatConfig resolves the settings the chat surface runs with, through the same
// ladder a cron job does: the user path first, then the file, the environment
// and the flags. Resolved once per process, because a daemon reads one config
// for its whole lifetime and a second read would let the adapter and the run
// disagree about the approval policy the process started under.
func ChatConfig() (settings.Config, error) {
	return chatConfigOnce()
}

func (t *streamTarget) recordDone(usage nacelle.Usage, text string) {
	if t.stats != nil {
		t.stats.Usage = usage
		t.stats.FinalContextTokens = usage.InputTokens + usage.CacheReadTokens + usage.CacheCreationTokens
	}
	if t.log != nil {
		t.log.Line(sessions.FromModel, text)
	}
}
