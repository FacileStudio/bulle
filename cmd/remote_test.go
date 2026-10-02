package cmd

import (
	"io"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestNewRemoteCmd(t *testing.T) {
	cmd := newRemoteCmd()
	if cmd.Use != "remote [host] [prompt]" {
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

func TestRunRemote_NoArgsShowsHelp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newRemoteCmd()
	cmd.SetOut(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected nil when showing help for no args, got %v", err)
	}
}

func TestRemoteListCmd(t *testing.T) {
	cmd := newRemoteListCmd()
	if cmd.Use != "list" || len(cmd.Aliases) == 0 || cmd.Aliases[0] != "ls" {
		t.Fatalf("unexpected list command: %s %v", cmd.Use, cmd.Aliases)
	}
	if err := printRemoteList(settings.Config{}, false); err != nil {
		t.Fatalf("printRemoteList error: %v", err)
	}
	if err := printRemoteList(settings.Config{}, true); err != nil {
		t.Fatalf("printRemoteList json error: %v", err)
	}
}

func TestBuildRemoteOptions_ConfigTarget(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{
		Targets: map[string]settings.RemoteTarget{"staging": {Host: "staging.internal", Port: 2222, User: "deploy", Root: "/srv/app"}},
	}}
	opts, err := buildRemoteOptions(&remoteFlags{}, cfg, []string{"staging", "run", "tests"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Target.Backend != "ssh" || opts.Target.Host != "staging.internal" || opts.Target.Port != 2222 {
		t.Fatalf("unexpected target: %+v", opts.Target)
	}
	if opts.User != "deploy" || opts.WorkDir != "/srv/app" || opts.PrintPrompt != "run tests" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestBuildRemoteOptions_FlagOverrides(t *testing.T) {
	f := remoteFlags{user: "override", workdir: "/custom", port: 2200, key: "/tmp/key"}
	opts, err := buildRemoteOptions(&f, settings.Config{}, []string{"user@10.0.0.5:22"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Target.User != "override" || opts.Target.Workdir != "/custom" || opts.Target.Port != 2200 || opts.Target.KeyPath != "/tmp/key" {
		t.Fatalf("expected flags to override the address, got %+v", opts.Target)
	}
}

func TestBuildRemoteOptions_MissingTarget(t *testing.T) {
	if _, err := buildRemoteOptions(&remoteFlags{}, settings.Config{}, nil); err == nil {
		t.Fatal("expected error when no remote target is available")
	}
}

func TestCollectRemoteEntries(t *testing.T) {
	cfg := settings.Config{Remote: settings.Remote{
		User: "deploy",
		Root: "/workspace",
		Targets: map[string]settings.RemoteTarget{
			"staging": {Host: "staging.internal", Port: 2222},
		},
	}}
	entries := collectRemoteEntries(cfg)
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if entries[0].Name != "staging" || entries[0].Host != "staging.internal" || entries[0].User != "deploy" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}
