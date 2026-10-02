package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runDetached(prompt string) error {
	if prompt == "" {
		return errors.New("no prompt: --detach requires a prompt")
	}
	exe, err := os.Executable()
	if err != nil {
		exe = "bulle"
	}
	args := buildDetachedArgs(prompt)
	cmd := exec.Command(exe, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	setDetachedProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("session detached in background (PID %d)\nresume with: bulle sessions attach %d\n", cmd.Process.Pid, cmd.Process.Pid)
	return nil
}

func buildDetachedArgs(prompt string) []string {
	var args []string
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		if a == "-d" || a == "--detach" || strings.HasPrefix(a, "-d=") || strings.HasPrefix(a, "--detach=") {
			continue
		}
		if a == prompt || a == "--print" || strings.HasPrefix(a, "--print=") {
			continue
		}
		args = append(args, a)
	}
	return append([]string{"--print", prompt}, args...)
}
