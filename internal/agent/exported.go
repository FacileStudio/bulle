package agent

import (
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/tui"
)

// RunSessionWithFlags runs an interactive agent session using the provided configuration flags.
func RunSessionWithFlags(v string, flags settings.Config) error {
	sess, cleanup, err := bootOrAskWithFlags(v, flags)
	if err != nil {
		return err
	}
	defer cleanup()
	return tui.Launch(*sess)
}

// RunHeadlessWithFlags runs a single prompt in headless mode with the provided configuration flags.
func RunHeadlessWithFlags(prompt string, flags settings.Config) error {
	prep, err := setupAgentToolsWithFlags(flags, false)
	if err != nil {
		return err
	}
	_, _, err = runHeadlessConfig(prompt, prep.config, nil)
	return err
}

// RunBench executes sequential headless benchmark timing runs.
func RunBench(prompt string, runs int, json bool) error {
	return runBench(benchFlags{prompt: prompt, runs: runs, json: json})
}

// StdinPrompt reads the first line of piped stdin when available.
func StdinPrompt() (string, error) {
	return stdinPrompt()
}
