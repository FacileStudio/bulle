package sandbox

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestTargetFromSSH(t *testing.T) {
	tgt := targetFromSSH("remote", sshEndpoint{host: "192.168.1.100", port: 2222, user: "deploy", keyPath: "/key", root: "/srv/app"})
	if tgt.Name != "remote" || tgt.Backend != "ssh" || tgt.Host != "192.168.1.100" || tgt.Port != 2222 {
		t.Fatalf("unexpected ssh target: %+v", tgt)
	}
	if tgt.User != "deploy" || tgt.KeyPath != "/key" || tgt.Workdir != "/srv/app" || tgt.Status != "running" {
		t.Fatalf("unexpected ssh target details: %+v", tgt)
	}
	if targetFromSSH("bare", sshEndpoint{}).Workdir != "" {
		t.Fatal("expected an empty workspace for an unset root")
	}
}

func TestResolveRemoteTarget_FromConfig(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{
		User:       "deploy",
		SSHKeyPath: "~/.ssh/id_ed25519",
		Root:       "/workspace",
		Targets: map[string]settings.RemoteTarget{
			"staging": {Host: "staging.example.com", Port: 2222, User: "admin", SSHKeyPath: "~/.ssh/staging", Root: "/srv/app"},
		},
	}}
	tgt, err := ResolveRemoteTarget("staging", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Backend != "ssh" || tgt.Host != "staging.example.com" || tgt.Port != 2222 || tgt.User != "admin" {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	if tgt.KeyPath != "~/.ssh/staging" || tgt.Workdir != "/srv/app" {
		t.Fatalf("unexpected target paths: %+v", tgt)
	}
}

func TestResolveRemoteTarget_InheritsGroupDefaults(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{
		User:       "deploy",
		Port:       2222,
		SSHKeyPath: "~/.ssh/id_ed25519",
		Root:       "/srv/app",
		Targets:    map[string]settings.RemoteTarget{"prod": {Host: "prod.internal"}},
	}}
	tgt, err := ResolveRemoteTarget("prod", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Host != "prod.internal" || tgt.Port != 2222 || tgt.User != "deploy" || tgt.Workdir != "/srv/app" {
		t.Fatalf("unexpected inherited target: %+v", tgt)
	}
}

func TestResolveRemoteTarget_DirectAddress(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{
		User:       "globaluser",
		SSHKeyPath: "~/.ssh/id_rsa",
		Root:       "/default/work",
	}}
	tgt, err := ResolveRemoteTarget("ubuntu@10.0.0.5:2222", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Backend != "ssh" || tgt.Host != "10.0.0.5" || tgt.Port != 2222 || tgt.User != "ubuntu" {
		t.Fatalf("unexpected parsed ssh target: %+v", tgt)
	}
	if tgt.Workdir != "/default/work" || tgt.KeyPath != "~/.ssh/id_rsa" {
		t.Fatalf("unexpected paths on parsed ssh target: %+v", tgt)
	}
}

// An alias with no configured port must stay zero: ssh then resolves it from
// ~/.ssh/config or its own default, which a forced -p 22 would override.
func TestResolveRemoteTarget_AliasLeavesPortToSSH(t *testing.T) {
	tgt, err := ResolveRemoteTarget("myserver", settings.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Host != "myserver" || tgt.Port != 0 {
		t.Fatalf("expected ssh_config alias to leave the port unset, got %+v", tgt)
	}
}

func TestResolveRemoteTarget_AliasUsesGroupPort(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{Port: 2222, User: "deploy"}}
	tgt, err := ResolveRemoteTarget("myserver", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Port != 2222 || tgt.User != "deploy" {
		t.Fatalf("expected group defaults to apply to an alias, got %+v", tgt)
	}
}

func TestResolveRemoteTarget_Default(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{
		Default: "prod",
		Targets: map[string]settings.RemoteTarget{"prod": {Host: "prod.internal"}},
	}}
	tgt, err := ResolveRemoteTarget("", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tgt.Name != "prod" || tgt.Host != "prod.internal" {
		t.Fatalf("unexpected default target resolved: %+v", tgt)
	}
	if _, err := ResolveRemoteTarget("", settings.Config{}); err == nil || !strings.Contains(err.Error(), "remote target is required") {
		t.Fatalf("expected required-target error, got %v", err)
	}
}
