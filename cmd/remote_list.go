package cmd

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

type remoteListEntry struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user,omitempty"`
	Root string `json:"root,omitempty"`
	Key  string `json:"key,omitempty"`
}

func collectRemoteEntries(cfg settings.Config) []remoteListEntry {
	entries := make([]remoteListEntry, 0, len(cfg.Remote.Targets))
	for name, tgt := range cfg.Remote.Targets {
		entries = append(entries, remoteListEntry{
			Name: name,
			Host: cmp.Or(tgt.Host, name),
			Port: cmp.Or(tgt.Port, cfg.Remote.Port, 22),
			User: cmp.Or(tgt.User, cfg.Remote.User),
			Root: cmp.Or(tgt.Root, cfg.Remote.Root),
			Key:  cmp.Or(tgt.SSHKeyPath, cfg.Remote.SSHKeyPath),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func printRemoteTextList(entries []remoteListEntry) {
	if len(entries) == 0 {
		fmt.Println("no remote hosts configured; pass user@host:port directly or add remote.targets in ~/.bulle.yml")
		return
	}
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green)
	for _, entry := range entries {
		fmt.Println(nameStyle.Render(entry.Name))
		fmt.Printf("host=%s\nport=%d\nuser=%s\nroot=%s\nkey=%s\n\n", entry.Host, entry.Port, entry.User, entry.Root, entry.Key)
	}
}

func newRemoteListCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured remote SSH hosts",
		Long:    "List every host configured under remote.targets in ~/.bulle.yml.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runRemoteList(jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the host list as JSON")
	return cmd
}

func runRemoteList(jsonOutput bool) error {
	cfg, err := settings.Settings("", settings.Config{})
	if err != nil {
		return err
	}
	return printRemoteList(cfg, jsonOutput)
}

func printRemoteList(cfg settings.Config, jsonOutput bool) error {
	entries := collectRemoteEntries(cfg)
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	printRemoteTextList(entries)
	return nil
}
