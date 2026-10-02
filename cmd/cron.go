package cmd

import (
	"os"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/spf13/cobra"
)

func newCronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron [command]",
		Short: "Manage scheduled headless agent jobs",
		Long: `Manage scheduled headless agent jobs that run from crontab.

Jobs are defined as YAML files in ~/.bulle/jobs/. Use one of the
subcommands to list, run, trust, install, or uninstall them. Each job can be
trusted with bulle cron trust <name> before it can be run or
installed. Use bulle cron install <name> to install a job into
the user's crontab.`,
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	cmd.AddCommand(newCronListCmd())
	cmd.AddCommand(newCronRunCmd())
	cmd.AddCommand(newCronTrustCmd())
	cmd.AddCommand(newCronInstallCmd())
	cmd.AddCommand(newCronUninstallCmd())
	return cmd
}

func newCronListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"status"},
		Short:   "List all registered cron jobs",
		Long:    "List all registered cron jobs from YAML files in ~/.bulle/jobs/, showing their armed state and trust status.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return agent.ListCronJobs()
		},
	}
}

func newCronRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Execute a cron job once in headless mode",
		Long:  "Run a specific cron job once in headless mode. The job runs without interactive prompts.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return agent.RunCronJob(args[0])
		},
	}
}

func newCronTrustCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trust <name>",
		Short: "Review and trust a job file",
		Long:  "Review a cron job file and approve its contents before running it.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return agent.TrustCronJob(args[0], os.Stdin)
		},
	}
}

func newCronInstallCmd() *cobra.Command {
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Install a cron job directly to crontab",
		Long:  "Install a configured and trusted cron job directly to the user's crontab.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return agent.InstallCronJobOptions(args[0], printOnly)
		},
	}
	cmd.Flags().BoolVarP(&printOnly, "print", "p", false, "print crontab entry without installing")
	return cmd
}

func newCronUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "uninstall <name>",
		Aliases: []string{"remove", "rm"},
		Short:   "Remove a cron job from crontab",
		Long:    "Remove an installed cron job from the user's crontab.",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return agent.UninstallCronJob(args[0])
		},
	}
}
