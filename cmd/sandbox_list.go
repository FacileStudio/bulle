package cmd

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/FacileStudio/bulle/internal/sandbox"
	"github.com/FacileStudio/bulle/internal/settings"
)

type sandboxListEntry struct {
	Name      string `json:"name"`
	VM        string `json:"vm,omitempty"`
	Backend   string `json:"backend"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port"`
	User      string `json:"user,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Key       string `json:"key,omitempty"`
	Status    string `json:"status,omitempty"`
}

// collectConfigTargetEntries lists the configured sandbox targets. It mirrors
// resolveConfigBoiteTarget's precedence — the instance's own port and identity
// first, then the target's overrides — so what this prints is what a session
// would actually connect to, rather than a second opinion built from the group
// defaults.
//
// Workspace is the directory the session's tools run in, resolved the same way
// the target resolves it. It is deliberately not the instance's recorded
// workspace: that one is a directory on this host, and printing it here is how
// a VM session came to look like it opened the launcher's home.
func collectConfigTargetEntries(cfg settings.Config, instances map[string]*sandbox.InstanceState) []sandboxListEntry {
	entries := make([]sandboxListEntry, 0, len(cfg.Sandbox.Targets))
	for name, tgt := range cfg.Sandbox.Targets {
		vm := cmp.Or(tgt.VMName, name)
		entry := sandboxListEntry{Name: name, VM: vm, Backend: "boite", Host: "127.0.0.1", User: "boite", Status: "configured"}
		if inst, ok := instances[vm]; ok {
			entry.Port = inst.SSHPort
			entry.Key = inst.KeyPath
			entry.Status = inst.Status
		}
		entry.Port = cmp.Or(tgt.Port, entry.Port, 2226)
		entry.User = cmp.Or(tgt.User, entry.User)
		entry.Workspace = cmp.Or(tgt.Root, cfg.Sandbox.Root, sandbox.GuestWorkspace)
		entry.Key = cmp.Or(tgt.SSHKeyPath, entry.Key, cfg.Sandbox.SSHKeyPath)
		entries = append(entries, entry)
	}
	return entries
}

func hasEntry(entries []sandboxListEntry, vm string) bool {
	for _, e := range entries {
		if cmp.Or(e.VM, e.Name) == vm {
			return true
		}
	}
	return false
}

func collectBoiteInstanceEntries(entries []sandboxListEntry, instances []*sandbox.InstanceState, cfg settings.Config) []sandboxListEntry {
	for _, inst := range instances {
		if hasEntry(entries, inst.Name) {
			continue
		}
		entries = append(entries, sandboxListEntry{
			Name:      inst.Name,
			VM:        inst.Name,
			Backend:   "boite",
			Host:      "127.0.0.1",
			Port:      cmp.Or(inst.SSHPort, 2226),
			Workspace: cmp.Or(cfg.Sandbox.Root, sandbox.GuestWorkspace),
			Key:       inst.KeyPath,
			Status:    inst.Status,
		})
	}
	return entries
}

// listInstances reads the local boite instances, or nothing when the instance
// directory is unreadable: a listing that cannot see them still has the
// configured targets to show.
func listInstances() []*sandbox.InstanceState {
	instances, err := sandbox.ListInstances()
	if err != nil {
		return nil
	}
	return instances
}

func printSandboxTextList(entries []sandboxListEntry) {
	if len(entries) == 0 {
		fmt.Println("no sandbox targets configured and no boite instances found")
		return
	}
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green)
	for _, entry := range entries {
		fmt.Println(nameStyle.Render(entry.Name))
		fmt.Printf("backend=%s\nvm=%s\nhost=%s\nport=%d\nstatus=%s\nworkspace=%s\nkey=%s\n\n",
			entry.Backend, entry.VM, entry.Host, entry.Port, entry.Status, entry.Workspace, entry.Key)
	}
}

func runSandboxList(jsonOutput bool) error {
	cfg, err := settings.Settings("", settings.Config{})
	if err != nil {
		return err
	}
	return printSandboxList(cfg, jsonOutput)
}

func printSandboxList(cfg settings.Config, jsonOutput bool) error {
	instances := listInstances()
	byName := make(map[string]*sandbox.InstanceState, len(instances))
	for _, inst := range instances {
		byName[inst.Name] = inst
	}
	entries := collectBoiteInstanceEntries(collectConfigTargetEntries(cfg, byName), instances, cfg)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	printSandboxTextList(entries)
	return nil
}
