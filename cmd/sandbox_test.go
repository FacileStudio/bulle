package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/sandbox"
	"github.com/FacileStudio/bulle/internal/settings"
)

func writeBoiteInstance(t *testing.T, dir, name string, port int) {
	t.Helper()
	instDir := filepath.Join(dir, name)
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatalf("creating instance dir: %v", err)
	}
	state := fmt.Sprintf(`{"name":%q,"ssh_port":%d,"key_path":"/key","status":"running","workspace":"/workspace"}`, name, port)
	if err := os.WriteFile(filepath.Join(instDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatalf("writing state.json: %v", err)
	}
}

func TestNewSandboxCmd(t *testing.T) {
	cmd := newSandboxCmd()
	if cmd.Use != "sandbox [vm] [prompt]" {
		t.Fatalf("unexpected Use: %s", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" || cmd.Example == "" {
		t.Fatal("expected non-empty Short, Long, and Example descriptions")
	}
	foundList := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			foundList = true
		}
	}
	if !foundList {
		t.Fatal("expected a list subcommand")
	}
}

func TestSandboxListCmd(t *testing.T) {
	cmd := newSandboxListCmd()
	if cmd.Use != "list" {
		t.Fatalf("unexpected Use: %s", cmd.Use)
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "ls" {
		t.Fatalf("expected alias 'ls', got %v", cmd.Aliases)
	}
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	if err := printSandboxList(settings.Config{}, false); err != nil {
		t.Fatalf("printSandboxList error: %v", err)
	}
	if err := printSandboxList(settings.Config{}, true); err != nil {
		t.Fatalf("printSandboxList json error: %v", err)
	}
}

func TestCollectConfigTargetEntries(t *testing.T) {
	cfg := settings.Config{Sandbox: settings.Sandbox{
		Root: "/workspace",
		Targets: map[string]settings.SandboxTarget{
			"dev-vm": {VMName: "dev-box", Port: 2230, User: "dev"},
		},
	}}
	entries := collectConfigTargetEntries(cfg, nil)
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if entries[0].Name != "dev-vm" || entries[0].VM != "dev-box" || entries[0].Port != 2230 || entries[0].Backend != "boite" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

// The instance names the port and key a session dials; the workspace it prints
// is the one the session's tools run in, which is the guest's own — the
// instance's recorded workspace is a directory on this host, never the guest's.
func TestCollectConfigTargetEntries_TakesInstanceValues(t *testing.T) {
	cfg := settings.Config{Sandbox: settings.Sandbox{
		Targets: map[string]settings.SandboxTarget{"dev-vm": {VMName: "dev-box"}},
	}}
	instances := map[string]*sandbox.InstanceState{
		"dev-box": {Name: "dev-box", SSHPort: 2240, KeyPath: "/inst/key", Workspace: "/home/yann", Status: "running"},
	}
	entries := collectConfigTargetEntries(cfg, instances)
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Port != 2240 || entry.Key != "/inst/key" || entry.Status != "running" {
		t.Fatalf("expected the instance's own port and key, got %+v", entry)
	}
	if entry.Workspace != sandbox.GuestWorkspace {
		t.Errorf("Workspace = %q, want %q", entry.Workspace, sandbox.GuestWorkspace)
	}
}

func TestBuildSandboxOptions_WithArgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOITE_INSTANCES_DIR", dir)
	writeBoiteInstance(t, dir, "testvm", 2226)
	cmd := newSandboxCmd()
	f := sandboxFlags{workdir: "/custom/dir", user: "customuser"}
	cfg := settings.Defaults("test")
	opts, err := buildSandboxOptions(cmd, &f, cfg, []string{"testvm", "hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Target.Name != "testvm" || opts.Target.Backend != "boite" {
		t.Fatalf("unexpected target: %+v", opts.Target)
	}
	if opts.WorkDir != "/custom/dir" || opts.User != "customuser" {
		t.Fatalf("unexpected overrides: %+v", opts)
	}
	if opts.PrintPrompt != "hello world" {
		t.Fatalf("expected prompt 'hello world', got %s", opts.PrintPrompt)
	}
}

func TestBuildSandboxOptions_SnapshotFromConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOITE_INSTANCES_DIR", dir)
	writeBoiteInstance(t, dir, "custom-vm", 2226)
	snapTrue := true
	cfg := settings.Defaults("test")
	cfg.Sandbox.Default = "custom-vm"
	cfg.Sandbox.AutoSnapshot = &snapTrue
	cmd := newSandboxCmd()
	opts, err := buildSandboxOptions(cmd, &sandboxFlags{}, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Snapshot {
		t.Fatal("expected Snapshot true from sandbox.auto_snapshot")
	}
}

func TestBuildSandboxOptions_MissingVM(t *testing.T) {
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	cmd := newSandboxCmd()
	f := sandboxFlags{}
	if _, err := buildSandboxOptions(cmd, &f, settings.Config{}, nil); err == nil {
		t.Fatal("expected error when no VM name is provided in args or config")
	}
}

func TestBuildSandboxOptions_ExplicitPrint(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOITE_INSTANCES_DIR", dir)
	writeBoiteInstance(t, dir, "custom-vm", 2226)
	cmd := newSandboxCmd()
	f := sandboxFlags{printPrompt: "run tests"}
	opts, err := buildSandboxOptions(cmd, &f, settings.Defaults("test"), []string{"custom-vm", "ignored", "positional"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.PrintPrompt != "run tests" {
		t.Fatalf("expected explicit print flag, got %s", opts.PrintPrompt)
	}
}

func TestRunSandbox_NoArgsShowsHelp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newSandboxCmd()
	cmd.SetOut(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected nil when showing help for no args, got %v", err)
	}
}

func TestRunSandbox_UsesDefaultTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	cfgPath := filepath.Join(home, ".bulle.yml")
	if err := os.WriteFile(cfgPath, []byte("sandbox:\n  default: ghost-vm\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cmd := newSandboxCmd()
	cmd.SetOut(io.Discard)
	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "no boite sandbox") {
		t.Fatalf("expected sandbox.default to be resolved, got %v", err)
	}
}
