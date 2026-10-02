package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/spf13/cobra"
)

type sessionsListFlags struct {
	json bool
	all  bool
}

func bindSessionsListFlags(cmd *cobra.Command, f *sessionsListFlags) {
	cmd.Flags().BoolVar(&f.json, "json", false, "Output session list in JSON format")
	cmd.Flags().BoolVarP(&f.all, "all", "a", false, "List all sessions across all workspaces")
}

func newSessionsListCmd(f *sessionsListFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active and past agent sessions",
		Long:    "List agent sessions with status, ID, PID, model, root, started time, and last activity.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSessionsList(f)
		},
	}
	bindSessionsListFlags(cmd, f)
	return cmd
}

func newListCmd() *cobra.Command {
	var f sessionsListFlags
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active and past agent sessions",
		Long:    "List agent sessions with status, ID, PID, model, root, started time, and last activity.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSessionsList(&f)
		},
	}
	bindSessionsListFlags(cmd, &f)
	return cmd
}

func runSessionsList(f *sessionsListFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	projectRoot := cwd
	if f.all {
		projectRoot = ""
	}
	list := sessions.ListSessions(projectRoot)
	if f.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}
	printSessionsTextList(list)
	return nil
}

func printSessionsTextList(entries []sessions.SessionInfo) {
	if len(entries) == 0 {
		fmt.Println("no sessions found")
		return
	}
	runningStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	stoppedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	idStyle := lipgloss.NewStyle().Bold(true)
	for _, s := range entries {
		badge := stoppedStyle.Render("○ stopped")
		if s.Status == "running" {
			badge = runningStyle.Render("● running")
		}
		fmt.Printf("%s  %s  PID %d\n", badge, idStyle.Render(s.ID), s.PID)
		fmt.Printf("  model: %s · root: %s\n", s.Model, s.Root)
		fmt.Printf("  started: %s · active: %s\n\n",
			s.Started.Format("2006-01-02 15:04:05"),
			s.ModTime.Format("2006-01-02 15:04:05"))
	}
}
