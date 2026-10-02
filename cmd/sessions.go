package cmd

import (
	"fmt"
	"os"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

func newSessionsCmd(version string) *cobra.Command {
	var f sessionsListFlags
	cmd := &cobra.Command{
		Use:     "sessions [command]",
		Aliases: []string{"session"},
		Short:   "Manage background and interactive agent sessions",
		Long: `Manage background and interactive bulle agent sessions.

Use 'bulle sessions list' (or 'bulle list') to view active and past sessions.
Use 'bulle sessions attach [id]' to resume an interactive session.
Use 'bulle sessions kill <id>' to terminate a running background session.
Use 'bulle sessions rm <id>' to delete a session.`,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}
	bindSessionsListFlags(cmd, &f)
	cmd.AddCommand(newSessionsListCmd(&f))
	cmd.AddCommand(newSessionsAttachCmd(version))
	cmd.AddCommand(newSessionsKillCmd())
	cmd.AddCommand(newSessionsDeleteCmd())
	return cmd
}

func newResumeCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:     "resume [id]",
		Aliases: []string{"attach"},
		Short:   "Resume an agent session interactively",
		Long:    "Attach to and resume an existing agent session by session ID or PID. If no ID is given, resumes the last session.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := ""
			if len(args) > 0 {
				id = args[0]
			}
			return runSessionsAttach(version, id)
		},
	}
}

func newSessionsAttachCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:     "attach [id]",
		Aliases: []string{"resume"},
		Short:   "Resume an agent session interactively",
		Long:    "Attach to and resume an existing agent session by session ID or PID. If no ID is given, resumes the last session.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := ""
			if len(args) > 0 {
				id = args[0]
			}
			return runSessionsAttach(version, id)
		},
	}
}

func newSessionsKillCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "kill <id>",
		Aliases: []string{"stop"},
		Short:   "Kill a running background session",
		Long:    "Terminate a running background agent session by session ID or PID.",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runSessionsKill(args[0])
		},
	}
}

func newSessionsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>",
		Aliases: []string{"delete", "remove"},
		Short:   "Delete a session file",
		Long:    "Permanently remove an agent session by session ID or PID.",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runSessionsDelete(args[0])
		},
	}
}

func runSessionsAttach(version string, id string) error {
	if id == "" {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		files := sessions.ListSessionFiles(cwd)
		if len(files) == 0 {
			files = sessions.ListSessionFiles("")
		}
		if len(files) == 0 {
			return fmt.Errorf("no session found to resume")
		}
		id = files[0]
	}
	cfg, err := settings.Settings("", settings.Config{})
	if err != nil {
		return err
	}
	cfg.Resume = &id
	return agent.RunSessionWithFlags(version, cfg)
}

func runSessionsKill(id string) error {
	session, err := sessions.GetSession(id)
	if err != nil {
		return err
	}
	if err := sessions.KillSession(id); err != nil {
		return err
	}
	fmt.Printf("killed session %s (PID %d)\n", id, session.PID)
	return nil
}

func runSessionsDelete(id string) error {
	if err := sessions.DeleteSession(id); err != nil {
		return err
	}
	fmt.Printf("deleted session %s\n", id)
	return nil
}
