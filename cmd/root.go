// Package cmd provides the command-line interface commands and flag parsing.
package cmd

import (
	"context"
	"errors"
	"os"
	"strings"

	"charm.land/fang/v2"
	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/spf13/cobra"
)

// Execute runs the bulle command tree through fang and returns the execution error.
func Execute(version string) error {
	agent.EnsureUserPath()
	normalizeArgs()
	root := newRootCmd(version)
	return fang.Execute(context.Background(), root, fang.WithVersion(version), fang.WithColorSchemeFunc(fang.DefaultColorScheme))
}

func newRootCmd(version string) *cobra.Command {
	var f cliFlags
	cmd := &cobra.Command{
		Use:   "bulle [flags] [prompt]",
		Short: "Terminal coding agent and harness for the nacelle SDK",
		Long: "bulle is a terminal coding agent built on the nacelle agent SDK.\n" +
			"It streams reasoning and answers in full-screen Bubble Tea or inline modes,\n" +
			"runs tools with optional approval, manages subagents, and schedules cron jobs.",
		DisableAutoGenTag: true,
		SilenceUsage:      true,
		SilenceErrors:     true,
		RunE: func(c *cobra.Command, args []string) error {
			return runRoot(c, &f, version, args)
		},
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	bindFlags(cmd, &f)
	cmd.AddCommand(newCronCmd())
	cmd.AddCommand(newChatCmd())
	cmd.AddCommand(newBenchCmd())
	cmd.AddCommand(newSandboxCmd())
	cmd.AddCommand(newRemoteCmd())
	cmd.AddCommand(newSessionsCmd(version))
	cmd.AddCommand(newResumeCmd(version))
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSessionsDeleteCmd())
	return cmd
}

func runRoot(c *cobra.Command, f *cliFlags, version string, args []string) error {
	cfg := collectFlags(c, f)
	prompt, isHeadless, err := resolvePrompt(c, f, args)
	if err != nil {
		return err
	}
	if f.detach {
		return runDetached(prompt)
	}
	if isHeadless {
		return agent.RunHeadlessWithFlags(prompt, cfg)
	}
	return agent.RunSessionWithFlags(version, cfg)
}

func resolvePrompt(c *cobra.Command, f *cliFlags, args []string) (string, bool, error) {
	printPassed := c.Flags().Changed("print")
	prompt := f.printPrompt
	if prompt == "" && len(args) > 0 {
		prompt = strings.Join(args, " ")
		printPassed = true
	}
	if printPassed && prompt == "" {
		piped, err := agent.StdinPrompt()
		if err != nil || piped == "" {
			return "", false, errors.New("no prompt: neither --print nor stdin provided")
		}
		prompt = piped
	}
	return prompt, printPassed, nil
}

func normalizeArgs() {
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			os.Args[i] = "-" + arg
		}
	}
}
