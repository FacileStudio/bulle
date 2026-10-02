package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/sandbox"
	"github.com/FacileStudio/bulle/internal/settings"
)

// checkPreflight verifies the target is reachable and isolated before any
// tool call is allowed to run against it.
func checkPreflight(ctx context.Context, target *sandbox.Target, opts sandbox.SessionOptions) error {
	if opts.SkipGuard {
		return nil
	}
	_, err := sandbox.PreflightCheck(ctx, target, sandbox.GuardOptions{
		ExpectedUser:    opts.User,
		ExpectedWorkdir: opts.WorkDir,
		Runner:          opts.Runner,
	})
	return err
}

func closeRemoteTools(closer io.Closer) {
	if closer == nil {
		return
	}
	if err := closer.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// splitTargetArg takes the target name off the front of the positional
// arguments, returning the words that follow as the prompt. Both commands
// accept the same shape, so the split lives here rather than in each.
func splitTargetArg(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

// resolvePromptArg picks the headless prompt: an explicit --print wins over the
// positional words after the target name.
func resolvePromptArg(printPrompt string, promptArgs []string) string {
	if printPrompt != "" {
		return printPrompt
	}
	return strings.Join(promptArgs, " ")
}

// targetSessionRoot is the root a target session records: the workspace on the
// target when one is configured, and otherwise a label naming the target, since
// the session then runs in the SSH login directory.
//
// It is never the host's working directory. Keeping that is how 0.69 put the
// launch directory into the banner, the system prompt and the session header of
// a session whose tools never touched this machine — and grouped its transcript
// with whatever local project bulle happened to be started from.
func targetSessionRoot(workdir, targetName string) string {
	if workdir != "" {
		return workdir
	}
	return settings.TargetRoot(targetName)
}

// runTargetSession boots bulle on the host against a resolved target, mounting
// the SSH-backed tool set in place of the local file and command tools.
func runTargetSession(ctx context.Context, version string, cfg settings.Config, opts sandbox.SessionOptions) error {
	if err := checkPreflight(ctx, opts.Target, opts); err != nil {
		return err
	}
	cfg.Root = targetSessionRoot(opts.WorkDir, opts.Target.Name)
	tools, closer, err := sandbox.RemoteTools(sandbox.ToolsOptions{
		Target:  opts.Target,
		WorkDir: opts.WorkDir,
		Runner:  opts.Runner,
	})
	if err != nil {
		return err
	}
	cleanup := func() { closeRemoteTools(closer) }
	if opts.PrintPrompt != "" {
		return agent.RunHeadlessWithTools(opts.PrintPrompt, cfg, tools, cleanup)
	}
	return agent.RunSessionWithTools(version, cfg, tools, cleanup)
}
