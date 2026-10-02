package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/ide"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/tui"
)

// RunSessionWithTools runs an interactive session with custom tools.
func RunSessionWithTools(v string, flags settings.Config, customTools []nacelle.Tool, cleanup func()) error {
	if cleanup != nil {
		defer cleanup()
	}
	sess, sessCleanup, err := bootOrAskWithTools(v, flags, customTools)
	if err != nil {
		return err
	}
	defer sessCleanup()
	return tui.Launch(*sess)
}

// RunHeadlessWithTools runs a headless prompt with custom tools.
func RunHeadlessWithTools(prompt string, flags settings.Config, customTools []nacelle.Tool, cleanup func()) error {
	if cleanup != nil {
		defer cleanup()
	}
	prep, err := setupAgentCustomTools(flags, customTools, false)
	if err != nil {
		return err
	}
	defer func() {
		if err := closeAll(prep.mcp.set); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	return runHeadlessTools(prompt, prep)
}

func setupAgentCustomTools(flags settings.Config, customTools []nacelle.Tool, noConfig bool) (preparedTools, error) {
	if noConfig {
		flags.NoConfig = &noConfig
	}
	config, err := settings.Settings(DefaultSystemPrompt(), flags)
	if err != nil {
		return preparedTools{}, err
	}
	reaching, err := webTools(config)
	if err != nil {
		return preparedTools{}, err
	}
	tools := append(slices.Clone(customTools), reaching...)
	mcp, tools, err := mcpTools(config, tools)
	if err != nil {
		return preparedTools{}, err
	}
	return preparedTools{config: config, mcp: mcp, local: tools}, nil
}

func buildUISessionWithTools(v string, flags settings.Config, customTools []nacelle.Tool, noConfig bool) (*tui.UISession, func(), error) {
	prep, err := setupAgentCustomTools(flags, customTools, noConfig)
	if err != nil {
		return nil, nil, err
	}
	sess, err := setupAgentSession(prep, v)
	if err != nil {
		return nil, nil, closeOnErr(err, prep.mcp.set)
	}
	return sess, func() {
		closeIDE(sess)
		if err := closeAll(prep.mcp.set); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}, nil
}

func bootOrAskWithTools(v string, flags settings.Config, customTools []nacelle.Tool) (*tui.UISession, func(), error) {
	sess, cleanup, err := buildUISessionWithTools(v, flags, customTools, false)
	if err == nil {
		return sess, cleanup, nil
	}
	var bad *settings.ParseError
	if !errors.As(err, &bad) {
		return nil, nil, err
	}
	fmt.Fprintln(os.Stderr, badConfigReport(bad))
	if !confirmDefaultSettings() {
		return nil, nil, refusedConfig(bad)
	}
	return buildUISessionWithTools(v, flags, customTools, true)
}

// startIDE opens the IDE surface when the session asked for it, by flag or by
// environment, and returns nil when it did not. A session with the surface off
// creates nothing at all — no socket, no discovery file, no goroutine — and
// merges no hook that would publish to a socket that does not exist.
//
// The model it announces is the resolved one rather than the raw setting: a
// config that leaves provider.model empty is the ordinary case, the provider
// fills its own default, and a hello naming no model would tell the editor less
// than the banner beside it. That name is read from the backend the session
// answers on, which is built before the agent is: the surface's hooks have to
// be in the map the agent is built from, so the surface cannot wait for the
// agent to exist.
func startIDE(config settings.Config, version string, backend nacelle.Backend) (*ide.Server, error) {
	if !ide.Enabled() {
		return nil, nil
	}
	return ide.Start(ide.Options{
		Root:    config.Root,
		Model:   resolvedModel(backend, config),
		Version: version,
	})
}

func buildHeadlessToolsAgent(p preparedTools, extra map[nacelle.HookPoint][]nacelle.Hook) (*nacelle.Agent, error) {
	augmentSystem(&p.config, p.mcp)
	_, approve := approval.Build(*p.config.ApproveTools)
	hooks, _, err := settings.SessionHooks(p.config)
	if err != nil {
		return nil, err
	}
	get, err := build(&p.config, p.local, approve, mergeHooks(hooks, extra))
	if err != nil {
		return nil, err
	}
	return get.agent, nil
}

func runHeadlessTools(prompt string, prep preparedTools) error {
	var stats runStats
	log := sessions.OpenSession(prep.config.Backend, prep.config.Model, prep.config.Root)
	log.Line(sessions.FromReader, prompt)
	agent, err := buildHeadlessToolsAgent(prep, stats.compactHook())
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	conv := []nacelle.Message{nacelle.UserText(prompt)}
	target := streamTarget{w: os.Stdout, stats: &stats, log: log}
	_, err = consumeHeadlessEvents(ctx, agent, conv, target)
	return err
}
