package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxDefaults(t *testing.T) {
	cfg := Defaults("test")
	if cfg.Sandbox.VMName != "" {
		t.Fatalf("expected VMName empty string, got %s", cfg.Sandbox.VMName)
	}
	if cfg.Sandbox.Port != 2226 {
		t.Fatalf("expected Port 2226, got %d", cfg.Sandbox.Port)
	}
	if cfg.Sandbox.Root != "" {
		t.Fatalf("expected empty Root, got %s", cfg.Sandbox.Root)
	}
	if cfg.Sandbox.AutoSnapshot == nil || *cfg.Sandbox.AutoSnapshot {
		t.Fatal("expected AutoSnapshot false")
	}
}

func TestRemoteDefaults(t *testing.T) {
	cfg := Defaults("test")
	if cfg.Remote.Port != 0 {
		t.Fatalf("expected Port 0 so ssh chooses, got %d", cfg.Remote.Port)
	}
	if cfg.Remote.Root != "" {
		t.Fatalf("expected empty Root, got %s", cfg.Remote.Root)
	}
	if cfg.Remote.SSHKeyPath != "" {
		t.Fatalf("expected empty identity so ssh chooses, got %s", cfg.Remote.SSHKeyPath)
	}
}

func testSandboxMergeObj() (Config, Config) {
	base := Defaults("test")
	snapVal := true
	over := Config{
		Sandbox: Sandbox{
			Default:      "dev-vm",
			User:         "deploy",
			VMName:       "tux",
			Port:         2230,
			SSHKeyPath:   "/custom/key",
			Root:         "/custom/root",
			AutoSnapshot: &snapVal,
		},
	}
	base.merge(over)
	return base, over
}

func TestSandboxMerge(t *testing.T) {
	base, _ := testSandboxMergeObj()
	if base.Sandbox.Default != "dev-vm" {
		t.Fatalf("expected Default dev-vm, got %s", base.Sandbox.Default)
	}
	if base.Sandbox.User != "deploy" {
		t.Fatalf("expected User deploy, got %s", base.Sandbox.User)
	}
	if base.Sandbox.VMName != "tux" || base.Sandbox.Port != 2230 {
		t.Fatalf("expected tux:2230, got %s:%d", base.Sandbox.VMName, base.Sandbox.Port)
	}
	if base.Sandbox.SSHKeyPath != "/custom/key" || base.Sandbox.Root != "/custom/root" {
		t.Fatalf("expected /custom/key and /custom/root, got %+v", base.Sandbox)
	}
	if base.Sandbox.AutoSnapshot == nil || !*base.Sandbox.AutoSnapshot {
		t.Fatal("expected AutoSnapshot true")
	}
}

func TestRemoteMerge(t *testing.T) {
	base := Defaults("test")
	over := Config{
		Remote: Remote{
			Default:    "staging",
			User:       "deploy",
			Port:       2222,
			SSHKeyPath: "/custom/key",
			Root:       "/srv/app",
		},
	}
	base.merge(over)
	if base.Remote.Default != "staging" || base.Remote.User != "deploy" {
		t.Fatalf("unexpected remote defaults: %+v", base.Remote)
	}
	if base.Remote.Port != 2222 || base.Remote.SSHKeyPath != "/custom/key" || base.Remote.Root != "/srv/app" {
		t.Fatalf("unexpected remote settings: %+v", base.Remote)
	}
}

func TestSandboxLoadYAML(t *testing.T) {
	yamlData := `sandbox:
  vm_name: arctic
  port: 2244
  ssh_key_path: /tmp/key
  root: /guest/ws
  auto_snapshot: true
remote:
  user: deploy
  port: 2222
  ssh_key_path: /tmp/remote_key
  root: /srv/app
`
	tmp := filepath.Join(t.TempDir(), ".bulle.yml")
	if err := os.WriteFile(tmp, []byte(yamlData), 0o644); err != nil {
		t.Fatalf("writing tmp config: %v", err)
	}
	loaded, err := Load(tmp)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if loaded.Sandbox.VMName != "arctic" || loaded.Sandbox.Port != 2244 {
		t.Fatalf("expected arctic:2244, got %s:%d", loaded.Sandbox.VMName, loaded.Sandbox.Port)
	}
	if loaded.Sandbox.AutoSnapshot == nil || !*loaded.Sandbox.AutoSnapshot {
		t.Fatal("expected AutoSnapshot true")
	}
	if loaded.Remote.User != "deploy" || loaded.Remote.Port != 2222 || loaded.Remote.Root != "/srv/app" {
		t.Fatalf("unexpected loaded remote config: %+v", loaded.Remote)
	}
}
