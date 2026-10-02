package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBaseMergeConfig() Config {
	snapFalse := false
	local := SandboxTarget{
		VMName:       "pingu",
		Port:         2226,
		Root:         "/workspace",
		AutoSnapshot: &snapFalse,
	}
	return Config{
		Sandbox: Sandbox{
			Default: "local",
			Targets: map[string]SandboxTarget{"local": local},
		},
	}
}

func testOverMergeConfig() Config {
	snapTrue := true
	local := SandboxTarget{
		Port:         2228,
		AutoSnapshot: &snapTrue,
		Root:         "/workspace/app",
	}
	cloud := SandboxTarget{
		VMName:     "cloud-box",
		Port:       22,
		User:       "ubuntu",
		SSHKeyPath: "~/.ssh/cloud_key",
	}
	return Config{
		Sandbox: Sandbox{
			Default: "cloud",
			Targets: map[string]SandboxTarget{
				"local": local,
				"cloud": cloud,
			},
		},
	}
}

func TestSandboxRemovedKeysRepointed(t *testing.T) {
	yamlData := "sandbox:\n  targets:\n    old:\n      backend: ssh\n      host: old.host\n"
	tmp := filepath.Join(t.TempDir(), ".bulle.yml")
	if err := os.WriteFile(tmp, []byte(yamlData), 0o644); err != nil {
		t.Fatalf("writing tmp config: %v", err)
	}
	_, err := Load(tmp)
	if _, ok := errors.AsType[*ParseError](err); !ok {
		t.Fatalf("expected a ParseError, got %v", err)
	}
	if !strings.Contains(err.Error(), "remote.targets") {
		t.Fatalf("expected the error to name remote.targets, got %v", err)
	}
}

func TestSandboxMergeTargets(t *testing.T) {
	base := testBaseMergeConfig()
	base.merge(testOverMergeConfig())

	if base.Sandbox.Default != "cloud" || len(base.Sandbox.Targets) != 2 {
		t.Fatalf("expected Default cloud and 2 targets: %+v", base.Sandbox)
	}
	local := base.Sandbox.Targets["local"]
	if local.VMName != "pingu" || local.Port != 2228 || local.Root != "/workspace/app" {
		t.Fatalf("unexpected local target: %+v", local)
	}
	if local.AutoSnapshot == nil || !*local.AutoSnapshot {
		t.Fatalf("expected merged AutoSnapshot true: %+v", local)
	}
	cloud := base.Sandbox.Targets["cloud"]
	if cloud.VMName != "cloud-box" || cloud.User != "ubuntu" || cloud.SSHKeyPath != "~/.ssh/cloud_key" {
		t.Fatalf("unexpected cloud target: %+v", cloud)
	}
}

func TestRemoteMergeTargets(t *testing.T) {
	base := Defaults("test")
	base.Remote.Targets = map[string]RemoteTarget{"prod": {Host: "old.internal", Port: 2200}}
	over := Config{Remote: Remote{
		Default: "prod",
		User:    "admin",
		Root:    "/srv/app",
		Targets: map[string]RemoteTarget{"prod": {Host: "new.internal", Root: "/srv/prod"}},
	}}
	base.merge(over)
	if base.Remote.Default != "prod" || base.Remote.User != "admin" || base.Remote.Root != "/srv/app" {
		t.Fatalf("unexpected remote group: %+v", base.Remote)
	}
	prod := base.Remote.Targets["prod"]
	if prod.Host != "new.internal" || prod.Port != 2200 || prod.Root != "/srv/prod" {
		t.Fatalf("unexpected merged remote target: %+v", prod)
	}
}

func testSandboxTargetYAML() string {
	return `sandbox:
  default: dev-vm
  user: globaluser
  vm_name: fallback-vm
  targets:
    local:
      vm_name: local-pingu
      port: 2226
      ssh_key_path: ~/.ssh/id_ed25519
      root: /workspace
      auto_snapshot: false
    dev-vm:
      vm_name: dev-box
      port: 2230
      user: dev
      ssh_key_path: ~/.ssh/custom_id
      root: /home/dev/work
      auto_snapshot: true
remote:
  default: staging
  user: deploy
  port: 22
  ssh_key_path: ~/.ssh/id_ed25519
  root: /workspace
  targets:
    staging:
      host: staging.example.com
      port: 2222
      user: admin
      ssh_key_path: ~/.ssh/staging_id
      root: /srv/app
`
}

func TestSandboxLoadYAMLWithTargets(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".bulle.yml")
	if err := os.WriteFile(tmp, []byte(testSandboxTargetYAML()), 0o644); err != nil {
		t.Fatalf("writing tmp config: %v", err)
	}
	loaded, err := Load(tmp)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if loaded.Sandbox.Default != "dev-vm" || loaded.Sandbox.User != "globaluser" {
		t.Fatalf("unexpected loaded sandbox config: %+v", loaded.Sandbox)
	}
	loc := loaded.Sandbox.Targets["local"]
	if loc.VMName != "local-pingu" || loc.Port != 2226 {
		t.Fatalf("unexpected local target fields: %+v", loc)
	}
	dev := loaded.Sandbox.Targets["dev-vm"]
	if dev.VMName != "dev-box" || dev.Port != 2230 || dev.User != "dev" || dev.Root != "/home/dev/work" {
		t.Fatalf("unexpected dev-vm target fields: %+v", dev)
	}
	if loaded.Remote.Default != "staging" || loaded.Remote.User != "deploy" {
		t.Fatalf("unexpected loaded remote group: %+v", loaded.Remote)
	}
	staging := loaded.Remote.Targets["staging"]
	if staging.Host != "staging.example.com" || staging.Port != 2222 || staging.User != "admin" {
		t.Fatalf("unexpected staging target fields: %+v", staging)
	}
}
