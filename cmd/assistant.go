package cmd

import (
	"fmt"

	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/spf13/cobra"
)

func newAssistantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assistant",
		Short: "Run the proactive assistant: watch, digests, memory consolidation",
		RunE:  func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newAssistantWatchCmd(), newAssistantDigestCmd(), newAssistantConsolidateCmd())
	cmd.AddCommand(newAssistantInstallCmd(), newAssistantStatusCmd())
	return cmd
}

func newAssistantWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Check mail, calendar and tasks and send an interrupt on a hard trigger",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runAssistantWatch(c.Context())
		},
	}
}

func newAssistantDigestCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "digest <morning|midday|evening>",
		Short:     "Send one of the daily digests",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: assistant.DigestNames(),
		RunE: func(c *cobra.Command, args []string) error {
			return runAssistantDigest(c.Context(), args[0])
		},
	}
}

func newAssistantConsolidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "consolidate",
		Short: "Consolidate yesterday's notes into memory and post what was kept",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runAssistantConsolidate(c.Context())
		},
	}
}

func newAssistantInstallCmd() *cobra.Command {
	var replace bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the assistant crontab entries",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runAssistantInstall(replace)
		},
	}
	cmd.Flags().BoolVar(&replace, "replace-legacy", false, "remove the assistant-pulse and assistant-checkin entries")
	return cmd
}

func newAssistantStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the last watch time, the ledger size and the next digests",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return printAssistantStatus()
		},
	}
}

func assistantDeps() (assistant.Deps, error) {
	cfg, err := assistantSettings()
	if err != nil {
		return assistant.Deps{}, fmt.Errorf("loading config: %w", err)
	}
	return assistant.Deps{Settings: cfg, Complete: assistantCompleter{profile: cfg.Profile}}, nil
}
