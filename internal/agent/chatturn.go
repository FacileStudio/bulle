package agent

import (
	"context"
	"io"
	"sync"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
)

var chatConfigOnce = sync.OnceValues(func() (settings.Config, error) {
	ensureUserPath()
	flags := settings.FromFlags(settings.Defaults(""))
	return settings.Settings(DefaultSystemPrompt(), flags)
})

// ChatConfig resolves the settings the chat surface runs with, through the same
// ladder a cron job does: the user path first, then the file, the environment
// and the flags. Resolved once per process, because a daemon reads one config
// for its whole lifetime and a second read would let the adapter and the run
// disagree about the approval policy the process started under.
func ChatConfig() (settings.Config, error) {
	return chatConfigOnce()
}

// ChatTurn runs one turn over the conversation so far and returns the answer
// text, without streaming anything to stdout: the daemon's stdout is a log a
// supervisor reads, not a chat. The caller's context is the only stop signal,
// so no handler is installed here.
//
// The run inherits config.ApproveTools untouched. buildHeadlessAgent wires no
// approval UI, and approval.Build(true) with no send returns false from Ask, so
// a tool that would need approval is refused rather than prompting into a void
// nobody is watching. Nothing on this path may widen the tool policy: an
// inbound message is untrusted text.
func ChatTurn(ctx context.Context, conv []nacelle.Message, config settings.Config) (string, error) {
	var stats runStats
	log := sessions.OpenSession(config.Backend, config.Model, config.Root)
	agent, cleanup, err := buildHeadlessAgent(config, mergeHooks(stats.compactHook(), nil))
	if err != nil {
		return "", err
	}
	defer cleanup()
	return consumeHeadlessEvents(ctx, agent, conv, streamTarget{w: io.Discard, stats: &stats, log: log})
}
