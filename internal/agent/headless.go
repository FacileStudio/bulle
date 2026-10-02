package agent

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"strings"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
)

// runHeadless runs a single prompt and streams text to stdout.
// The prompt comes from the argument, or from stdin when piped.
// Exit codes: 0 on clean completion, 1 on error.
func runHeadless(prompt string) error {
	flags := settings.FromFlags(settings.Defaults(""))
	config, err := settings.Settings(DefaultSystemPrompt(), flags)
	if err != nil {
		return err
	}
	_, _, err = runHeadlessConfig(prompt, config, nil)
	return err
}

// runHeadlessConfig streams one prompt through an agent built from the given
// config and returns the full text plus what the run measured, streaming the
// text to stdout. Extra hooks beyond the run's own compaction counter ride
// along — a cron run adds its job's hooks here. The caller decides what to do
// with the results — the -print path drops both, a cron run delivers the text
// and records the stats.
func runHeadlessConfig(prompt string, config settings.Config, extra map[nacelle.HookPoint][]nacelle.Hook) (string, runStats, error) {
	return runHeadlessConfigToContext(context.Background(), os.Stdout, prompt, config, extra)
}

type streamTarget struct {
	w     io.Writer
	stats *runStats
	log   *sessions.SessionLog
}

func runHeadlessConfigToContext(parent context.Context, w io.Writer, prompt string, config settings.Config, extra map[nacelle.HookPoint][]nacelle.Hook) (string, runStats, error) {
	var stats runStats
	log := sessions.OpenSession(config.Backend, config.Model, config.Root)
	log.Line(sessions.FromReader, prompt)

	agent, cleanup, err := buildHeadlessAgent(config, mergeHooks(stats.compactHook(), extra))
	if err != nil {
		return "", stats, err
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()

	conv := []nacelle.Message{nacelle.UserText(prompt)}
	out, err := consumeHeadlessEvents(ctx, agent, conv, streamTarget{w: w, stats: &stats, log: log})
	return out, stats, err
}

func consumeHeadlessEvents(ctx context.Context, agent *nacelle.Agent, conv []nacelle.Message, target streamTarget) (string, error) {
	var out strings.Builder
	for event, err := range agent.Stream(ctx, conv) {
		if err != nil {
			return "", err
		}
		switch event.Kind {
		case nacelle.KindText:
			if _, err := fmt.Fprint(target.w, event.Text); err != nil {
				return "", err
			}
			out.WriteString(event.Text)
		case nacelle.KindToolCall:
			target.stats.ToolCalls++
		case nacelle.KindTurn:
			target.stats.FinalContextTokens = event.Usage.InputTokens + event.Usage.CacheReadTokens + event.Usage.CacheCreationTokens
		case nacelle.KindDone:
			target.stats.Usage = event.Usage
			target.log.Line(sessions.FromModel, out.String())
		}
	}
	if _, err := fmt.Fprintln(target.w); err != nil {
		return "", err
	}
	out.WriteString("\n")
	return out.String(), nil
}

// buildHeadlessAgent assembles the agent the same way the TUI does,
// without approval-gate wiring or banner construction. It returns the
// agent and a cleanup function the caller must defer. Extra hooks ride
// the settings hooks.
func buildHeadlessAgent(config settings.Config, extra map[nacelle.HookPoint][]nacelle.Hook) (*nacelle.Agent, func(), error) {
	set, local, err := localTools(config)
	if err != nil {
		return nil, nil, err
	}

	mcp, local, err := mcpTools(config, local)
	if err != nil {
		return nil, nil, closeOnErr(err, set)
	}

	augmentSystem(&config, mcp)
	_, approve := approval.Build(*config.ApproveTools)

	hooks, _, err := settings.SessionHooks(config)
	if err != nil {
		return nil, nil, closeOnErr(err, set, mcp.set)
	}

	get, err := build(&config, local, approve, mergeHooks(hooks, extra))
	if err != nil {
		return nil, nil, closeOnErr(err, set, mcp.set)
	}

	return get.agent, func() {
		if err := closeAll(set, mcp.set); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}, nil
}

// mergeHooks returns the settings hooks with the caller's extra hooks
// appended, copying on write so both inputs stay untouched.
func mergeHooks(hooks, extra map[nacelle.HookPoint][]nacelle.Hook) map[nacelle.HookPoint][]nacelle.Hook {
	out := maps.Clone(hooks)
	if out == nil {
		out = map[nacelle.HookPoint][]nacelle.Hook{}
	}
	for point, hs := range extra {
		out[point] = append(out[point], hs...)
	}
	return out
}

// closeOnErr returns the original err if cleanup succeeds, or the cleanup
// error if cleanup fails. The caller should prefer the cleanup error only
// when it wants to surface close failures over the original failure.
func closeOnErr(err error, closers ...any) error {
	if err == nil {
		return nil
	}
	if cerr := closeAll(closers...); cerr != nil {
		return cerr
	}
	return err
}

// closeAll calls Close on every closer it receives. It returns the last
// error returned by a Close() error call, if any.
func closeAll(closers ...any) error {
	var lastErr error
	for _, c := range closers {
		if c == nil {
			continue
		}
		switch v := c.(type) {
		case interface{ Close() error }:
			if err := v.Close(); err != nil {
				lastErr = err
			}
		case interface{ Close() }:
			v.Close()
		}
	}
	return lastErr
}
