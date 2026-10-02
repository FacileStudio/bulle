package cmd

import (
	"github.com/spf13/cobra"

	"github.com/FacileStudio/bulle/internal/sandbox"
	"github.com/FacileStudio/bulle/internal/settings"
)

type remoteFlags struct {
	workdir     string
	user        string
	port        int
	key         string
	printPrompt string
}

func bindRemoteFlags(cmd *cobra.Command, f *remoteFlags) {
	fl := cmd.Flags()
	fl.StringVarP(&f.workdir, "workdir", "w", "", "Workspace directory on the remote host")
	fl.StringVarP(&f.user, "user", "u", "", "SSH user for the remote host")
	fl.IntVar(&f.port, "port", 0, "SSH port for the remote host, overriding remote.port")
	fl.StringVar(&f.key, "key", "", "SSH identity file for the remote host")
	fl.StringVarP(&f.printPrompt, "print", "p", "", "Run this prompt headlessly and stream the output")
}

func buildRemoteOptions(f *remoteFlags, cfg settings.Config, args []string) (sandbox.SessionOptions, error) {
	targetName, promptArgs := splitTargetArg(args)
	target, err := sandbox.ResolveRemoteTarget(targetName, cfg)
	if err != nil {
		return sandbox.SessionOptions{}, err
	}
	if f.user != "" {
		target.User = f.user
	}
	if f.workdir != "" {
		target.Workdir = f.workdir
	}
	if f.port > 0 {
		target.Port = f.port
	}
	if f.key != "" {
		target.KeyPath = f.key
	}
	return sandbox.SessionOptions{
		Target:      target,
		WorkDir:     target.Workdir,
		User:        target.User,
		PrintPrompt: resolvePromptArg(f.printPrompt, promptArgs),
	}, nil
}
