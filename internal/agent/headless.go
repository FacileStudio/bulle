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
	"github.com/FacileStudio/bulle/internal/engine"
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

	agent, cleanup, err := BuildHeadlessAgent(config, mergeHooks(stats.compactHook(), extra))
	if err != nil {
		return "", stats, err
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()

	sess := engine.NewSession(agent, nil)
	events, err := sess.Submit(ctx, prompt)
	if err != nil {
		return "", stats, err
	}
	out, err := consumeHeadlessEvents(ctx, events, streamTarget{w: w, stats: &stats, log: log})
	return out, stats, err
}

func consumeHeadlessEvents(ctx context.Context, events <-chan engine.Event, target streamTarget) (string, error) {
	if target.w == nil {
		target.w = io.Discard
	}
	if target.stats == nil {
		target.stats = &runStats{}
	}
	var out strings.Builder
	for event := range events {
		switch event.Kind {
		case engine.EventError:
			return "", event.Err
		case engine.EventTextDelta:
			if _, err := fmt.Fprint(target.w, event.Text); err != nil {
				return "", err
			}
			out.WriteString(event.Text)
		case engine.EventThinking:
			fmt.Fprint(os.Stderr, event.Text)
		case engine.EventToolCall:
			target.stats.ToolCalls++
			fmt.Fprintf(os.Stderr, "\n> Tool: %s()\n", event.Tool.Name)
		case engine.EventToolResult:
			fmt.Fprintln(os.Stderr, "> Done")
		case engine.EventTurnDone:
			target.recordDone(event.Usage, out.String())
		}
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if _, err := fmt.Fprintln(target.w); err != nil {
		return "", err
	}
	out.WriteString("\n")
	return out.String(), nil
}

// BuildHeadlessAgent assembles the agent the same way the TUI does,
// without approval-gate wiring or banner construction. It returns the
// agent and a cleanup function the caller must defer. Extra hooks ride
// the settings hooks.
func BuildHeadlessAgent(config settings.Config, extra map[nacelle.HookPoint][]nacelle.Hook) (*nacelle.Agent, func(), error) {
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
