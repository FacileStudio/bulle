package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/bulle/internal/sandbox"
	"github.com/FacileStudio/bulle/internal/settings"
)

// resolveSandboxSnapshot picks the snapshot decision: an explicit --snapshot
// flag wins over the target's own setting, which wins over the group default.
func resolveSandboxSnapshot(cmd *cobra.Command, f *sandboxFlags, target *sandbox.Target, cfg settings.Config) bool {
	if cmd.Flags().Changed("snapshot") {
		return f.snapshot
	}
	if target != nil && target.AutoSnapshot != nil {
		return *target.AutoSnapshot
	}
	if cfg.Sandbox.AutoSnapshot != nil {
		return *cfg.Sandbox.AutoSnapshot
	}
	return false
}

func bindSandboxFlags(cmd *cobra.Command, f *sandboxFlags) {
	fl := cmd.Flags()
	fl.BoolVar(&f.snapshot, "snapshot", false, "Snapshot the VM overlay disk on exit (boite only)")
	fl.StringVarP(&f.workdir, "workdir", "w", "", "Workspace directory inside the VM")
	fl.StringVarP(&f.user, "user", "u", "", "SSH user inside the VM")
	fl.StringVarP(&f.printPrompt, "print", "p", "", "Run this prompt headlessly and stream the output")
}

func buildSandboxOptions(cmd *cobra.Command, f *sandboxFlags, cfg settings.Config, args []string) (sandbox.SessionOptions, error) {
	targetName, promptArgs := splitTargetArg(args)
	target, err := sandbox.ResolveTarget(targetName, cfg)
	if err != nil {
		return sandbox.SessionOptions{}, err
	}
	if f.user != "" {
		target.User = f.user
	}
	if f.workdir != "" {
		target.Workdir = f.workdir
	}
	return sandbox.SessionOptions{
		Target:      target,
		WorkDir:     target.Workdir,
		User:        target.User,
		Snapshot:    resolveSandboxSnapshot(cmd, f, target, cfg),
		PrintPrompt: resolvePromptArg(f.printPrompt, promptArgs),
	}, nil
}

func triggerSnapshotIfRequested(ctx context.Context, target *sandbox.Target, opts sandbox.SessionOptions) error {
	if !opts.Snapshot || target == nil || target.Backend != "boite" {
		return nil
	}
	tag := fmt.Sprintf("session-%d", time.Now().Unix())
	if err := sandbox.TakeSnapshot(ctx, target.Name, tag, sandbox.SnapshotOptions{Runner: opts.Runner}); err != nil {
		return fmt.Errorf("snapshot error: %w", err)
	}
	return nil
}
