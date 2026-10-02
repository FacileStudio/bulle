package agent

import (
	"errors"
	"fmt"
	"os"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/tui"
)

func setupAgentToolsWithFlags(flags settings.Config, noConfig bool) (preparedTools, error) {
	if noConfig {
		flags.NoConfig = &noConfig
	}
	config, err := settings.Settings(DefaultSystemPrompt(), flags)
	if err != nil {
		return preparedTools{}, err
	}
	set, local, err := localTools(config)
	if err != nil {
		return preparedTools{}, err
	}
	mcp, local, err := mcpTools(config, local)
	if err != nil {
		return preparedTools{}, closeOnErr(err, set)
	}
	return preparedTools{config: config, set: set, mcp: mcp, local: local}, nil
}

func buildUISessionWithFlags(v string, flags settings.Config, noConfig bool) (*tui.UISession, func(), error) {
	prep, err := setupAgentToolsWithFlags(flags, noConfig)
	if err != nil {
		return nil, nil, err
	}
	sess, err := setupAgentSession(prep, v)
	if err != nil {
		return nil, nil, closeOnErr(err, prep.set, prep.mcp.set)
	}
	return sess, func() {
		closeIDE(sess)
		if err := closeAll(prep.set, prep.mcp.set); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}, nil
}

func bootOrAskWithFlags(v string, flags settings.Config) (*tui.UISession, func(), error) {
	sess, cleanup, err := buildUISessionWithFlags(v, flags, false)
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
	return buildUISessionWithFlags(v, flags, true)
}
