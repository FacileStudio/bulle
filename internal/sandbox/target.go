package sandbox

import (
	"cmp"
	"errors"
	"fmt"

	"github.com/FacileStudio/bulle/internal/settings"
)

// Target is a resolved SSH execution endpoint: where a session's tools run.
// A boite target points at a local microVM on loopback; a remote target points
// at a configured SSH host. Both are reached the same way, which is why one
// type serves the two commands.
type Target struct {
	Name         string `json:"name" yaml:"name"`
	Backend      string `json:"backend" yaml:"backend"`
	Host         string `json:"host" yaml:"host"`
	Port         int    `json:"port" yaml:"port"`
	User         string `json:"user" yaml:"user"`
	KeyPath      string `json:"key_path" yaml:"key_path"`
	Workdir      string `json:"workdir" yaml:"workdir"`
	Status       string `json:"status" yaml:"status"`
	AutoSnapshot *bool  `json:"auto_snapshot,omitempty" yaml:"auto_snapshot,omitempty"`
}

// GuestWorkspace is the directory a boite VM always presents the project at:
// the base image bakes it in and workspace sync replaces it wholesale, so it is
// the one guest path a session can be started in.
//
// It exists as a constant because InstanceState.Workspace is *not* that path.
// boite records there the host directory it exports — the cwd `boite create`
// ran in — and a guest has no such directory unless the host tree happens to
// be laid out identically. Passing it to the preflight guard is the 0.70 bug:
// every VM session refused with `project directory "/home/yann" does not exist
// in target`, naming the launcher's home as if the guest had one, while the
// tools would have run in /workspace all along. An instance with no_mount still
// has /workspace; only its contents go stale, which is the guest's business.
const GuestWorkspace = "/workspace"

// TargetFromInstance creates a Target configured from a Boite InstanceState.
func TargetFromInstance(st *InstanceState) *Target {
	if st == nil {
		return nil
	}
	port := st.SSHPort
	if port <= 0 {
		port = 2226
	}
	return &Target{
		Name:    st.Name,
		Backend: "boite",
		Host:    "127.0.0.1",
		Port:    port,
		User:    "boite",
		KeyPath: st.KeyPath,
		Workdir: GuestWorkspace,
		Status:  st.Status,
	}
}

func resolveConfigBoiteTarget(name string, t settings.SandboxTarget, cfg settings.Config) (*Target, error) {
	vmName := cmp.Or(t.VMName, name)
	inst, err := LoadInstance(vmName)
	if err != nil {
		return nil, fmt.Errorf("loading boite instance %s: %w", vmName, err)
	}
	target := TargetFromInstance(inst)
	if t.User != "" {
		target.User = t.User
	}
	if t.Port > 0 {
		target.Port = t.Port
	}
	target.KeyPath = cmp.Or(t.SSHKeyPath, target.KeyPath, cfg.Sandbox.SSHKeyPath)
	target.Workdir = cmp.Or(t.Root, cfg.Sandbox.Root, target.Workdir)
	target.AutoSnapshot = t.AutoSnapshot
	return target, nil
}

func resolveTargetFromConfig(name string, cfg settings.Config) (*Target, bool, error) {
	t, ok := cfg.Sandbox.Targets[name]
	if !ok {
		return nil, false, nil
	}
	target, err := resolveConfigBoiteTarget(name, t, cfg)
	return target, true, err
}

// resolveTargetFromInstance builds a target from a local boite instance. The
// instance contributes its transport and identity, not a workspace: where a
// guest keeps the project is boite's own contract (GuestWorkspace), and the
// directory its state records is a path on this host. An explicit sandbox.root
// still wins, because pointing a VM at a directory you made there is the user's
// call — that is the one override the group root was always meant to be.
func resolveTargetFromInstance(name string, cfg settings.Config) (*Target, bool) {
	inst, err := LoadInstance(name)
	if err != nil || inst == nil {
		return nil, false
	}
	target := TargetFromInstance(inst)
	if cfg.Sandbox.User != "" {
		target.User = cfg.Sandbox.User
	}
	if target.KeyPath == "" {
		target.KeyPath = cfg.Sandbox.SSHKeyPath
	}
	target.Workdir = cmp.Or(cfg.Sandbox.Root, target.Workdir)
	if cfg.Sandbox.AutoSnapshot != nil {
		target.AutoSnapshot = cfg.Sandbox.AutoSnapshot
	}
	return target, true
}

// ResolveTarget resolves a boite sandbox by name: an entry in sandbox.targets
// first, then a local boite instance of that name. A name that matches neither
// is an error rather than an SSH address — SSH hosts belong to `bulle remote`.
func ResolveTarget(name string, cfg settings.Config) (*Target, error) {
	targetName := cmp.Or(name, cfg.Sandbox.Default, cfg.Sandbox.VMName)
	if targetName == "" {
		return nil, errors.New("sandbox target is required: pass a boite VM name or set sandbox.default in settings")
	}
	if target, found, err := resolveTargetFromConfig(targetName, cfg); found {
		return target, err
	}
	if target, found := resolveTargetFromInstance(targetName, cfg); found {
		return target, nil
	}
	return nil, fmt.Errorf("no boite sandbox %q: not a sandbox.targets entry or a local boite instance (try: bulle sandbox list)", targetName)
}
