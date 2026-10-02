package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

// TestTargetFromInstance pins the fix for the 0.70 VM refusal: the workspace
// field of an instance's state is the host directory boite exports, so the
// target must carry the guest's own workspace and not that path. Handing the
// host path on made every sandbox session die in the preflight guard with
// `project directory "/home/yann" does not exist in target`.
func TestTargetFromInstance(t *testing.T) {
	if TargetFromInstance(nil) != nil {
		t.Fatal("expected nil for nil instance")
	}
	st := &InstanceState{
		Name:      "testvm",
		SSHPort:   2226,
		KeyPath:   "/path/to/key",
		Workspace: "/home/yann",
		Status:    "running",
	}
	tgt := TargetFromInstance(st)
	if tgt.Name != "testvm" || tgt.Backend != "boite" || tgt.Host != "127.0.0.1" || tgt.Port != 2226 {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	if tgt.User != "boite" || tgt.KeyPath != "/path/to/key" || tgt.Status != "running" {
		t.Fatalf("unexpected target details: %+v", tgt)
	}
	if tgt.Workdir != GuestWorkspace {
		t.Errorf("Workdir = %q, want %q: a host path is not a directory the guest has", tgt.Workdir, GuestWorkspace)
	}
}

func writeBoiteInstance(t *testing.T, name string, port int, status, workspace string) {
	t.Helper()
	dir, err := InstancesDir()
	if err != nil {
		t.Fatalf("instances dir: %v", err)
	}
	instDir := filepath.Join(dir, name)
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatalf("creating instance dir: %v", err)
	}
	state := fmt.Sprintf(`{"name":%q,"ssh_port":%d,"key_path":"/key","status":%q,"workspace":%q}`, name, port, status, workspace)
	if err := os.WriteFile(filepath.Join(instDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatalf("writing state.json: %v", err)
	}
}

func TestResolveTarget_FromConfigTargets(t *testing.T) {
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	writeBoiteInstance(t, "dev-box", 2226, "running", "/workspace")
	devVM := settings.SandboxTarget{
		VMName:     "dev-box",
		Port:       2230,
		User:       "dev",
		SSHKeyPath: "~/.ssh/custom",
		Root:       "/home/dev/work",
	}
	cfg := settings.Config{Sandbox: settings.Sandbox{Targets: map[string]settings.SandboxTarget{"dev-vm": devVM}}}
	tgt, err := ResolveTarget("dev-vm", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Backend != "boite" || tgt.Port != 2230 || tgt.User != "dev" {
		t.Fatalf("unexpected resolved target: %+v", tgt)
	}
	if tgt.Workdir != "/home/dev/work" || tgt.KeyPath != "~/.ssh/custom" {
		t.Fatalf("unexpected resolved target paths: %+v", tgt)
	}
}

// TestResolveTarget_FromBoiteInstance covers the instance route end to end: the
// guest workspace is where a session starts, and an explicit sandbox.root is
// the one thing allowed to override it.
func TestResolveTarget_FromBoiteInstance(t *testing.T) {
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	writeBoiteInstance(t, "local-box", 2230, "running", "/home/yann")
	tgt, err := ResolveTarget("local-box", settings.Defaults("test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Backend != "boite" || tgt.Port != 2230 || tgt.Workdir != GuestWorkspace {
		t.Fatalf("unexpected boite target: %+v", tgt)
	}

	cfg := settings.Defaults("test")
	cfg.Sandbox.Root = "/srv/app"
	overridden, err := ResolveTarget("local-box", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overridden.Workdir != "/srv/app" {
		t.Errorf("Workdir = %q, want sandbox.root to win over the guest default", overridden.Workdir)
	}
}

func TestResolveTarget_Default(t *testing.T) {
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	writeBoiteInstance(t, "dev-box", 2226, "running", "/workspace")
	cfg := settings.Config{Sandbox: settings.Sandbox{Default: "dev-box"}}
	tgt, err := ResolveTarget("", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Name != "dev-box" || tgt.Backend != "boite" {
		t.Fatalf("unexpected default target resolved: %+v", tgt)
	}
}

func TestResolveTarget_UnknownIsError(t *testing.T) {
	t.Setenv("BOITE_INSTANCES_DIR", t.TempDir())
	if _, err := ResolveTarget("not-a-vm", settings.Config{}); err == nil || !strings.Contains(err.Error(), "no boite sandbox") {
		t.Fatalf("expected unknown-target error, got %v", err)
	}
	if _, err := ResolveTarget("", settings.Config{}); err == nil {
		t.Fatal("expected error when no target name is available")
	}
}
