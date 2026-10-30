package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

func TestSessionsHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sessions", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on sessions help, got: %v", err)
	}
	for _, want := range []string{"list", "attach", "kill"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("expected %q in sessions help, got: %s", want, buf.String())
		}
	}
}

func TestSessionsNoArgsShowsHelp(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"sessions"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error running sessions with no args, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Manage background and interactive")) {
		t.Errorf("expected help text when running sessions with no args, got: %s", buf.String())
	}
}

func TestRootResumeAndAttachSubcommands(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	resumeCmd, _, err := cmd.Find([]string{"resume"})
	if err != nil || resumeCmd == nil || resumeCmd.Name() != "resume" {
		t.Fatalf("expected resume subcommand on root, got %v (err: %v)", resumeCmd, err)
	}
	attachCmd, _, err := cmd.Find([]string{"attach"})
	if err != nil || attachCmd == nil || attachCmd.Name() != "resume" {
		t.Fatalf("expected attach alias on root resume command, got %v (err: %v)", attachCmd, err)
	}
}

func TestListHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on list help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("--json")) {
		t.Errorf("expected --json in list help, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("--all")) {
		t.Errorf("expected --all in list help, got: %s", buf.String())
	}
}

func TestDetachFlagExists(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	if cmd.Flags().Lookup("detach") == nil {
		t.Error("expected --detach flag on root command")
	}
	if cmd.Flags().ShorthandLookup("d") == nil {
		t.Error("expected -d shorthand flag on root command")
	}
}

func TestBuildDetachedArgs(t *testing.T) {
	args := buildDetachedArgs("hello world")
	if len(args) < 2 {
		t.Fatalf("expected at least 2 args, got %v", args)
	}
	if args[0] != "--print" || args[1] != "hello world" {
		t.Errorf("expected --print 'hello world', got %v", args)
	}
}

func TestResumeNoSessionsReturnsError(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"resume"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no session found to resume") {
		t.Fatalf("expected 'no session found to resume', got %v", err)
	}
}
