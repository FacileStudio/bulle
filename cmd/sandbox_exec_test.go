package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/bulle/internal/sandbox"
	"github.com/FacileStudio/bulle/internal/settings"
)

func TestRootVersion(t *testing.T) {
	if rootVersion(nil) != "" {
		t.Fatal("expected empty string for nil command")
	}
	root := &cobra.Command{Use: "root", Version: "1.2.3"}
	sub := &cobra.Command{Use: "sub"}
	root.AddCommand(sub)
	if rootVersion(sub) != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %s", rootVersion(sub))
	}
}

func TestTriggerSnapshotIfRequested(t *testing.T) {
	ctx := context.Background()
	if err := triggerSnapshotIfRequested(ctx, nil, sandbox.SessionOptions{Snapshot: false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sshTarget := &sandbox.Target{Name: "host", Backend: "ssh"}
	if err := triggerSnapshotIfRequested(ctx, sshTarget, sandbox.SessionOptions{Snapshot: true}); err != nil {
		t.Fatalf("unexpected error for non-boite backend: %v", err)
	}
}

// A target session must never record the directory bulle was launched from: the
// model's tools run on the other machine, and a host path there is a claim
// about a filesystem the session never opened.
func TestTargetSessionRootNamesTheTargetWithoutAWorkdir(t *testing.T) {
	if got := targetSessionRoot("/srv/app", "staging"); got != "/srv/app" {
		t.Errorf("targetSessionRoot with a workdir = %q, want the workspace on the target", got)
	}
	if got := targetSessionRoot("", "pingu"); got != "target:pingu" {
		t.Errorf("targetSessionRoot without a workdir = %q, want the target label", got)
	}
}

func TestRunTargetSession_PreflightFailure(t *testing.T) {
	ctx := context.Background()
	target := &sandbox.Target{
		Name:    "pingu",
		Backend: "boite",
		Status:  "stopped",
	}
	cfg := settings.Defaults("test")
	opts := sandbox.SessionOptions{
		Target:  target,
		WorkDir: "/workspace",
		User:    "boite",
	}
	err := runTargetSession(ctx, "0.0.1", cfg, opts)
	if err == nil || !strings.Contains(err.Error(), "must be running") {
		t.Fatalf("expected stopped error from preflight, got %v", err)
	}
}
