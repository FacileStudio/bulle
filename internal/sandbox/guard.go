package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// GuardOptions configures pre-flight isolation guard verification.
type GuardOptions struct {
	ExpectedUser    string
	ExpectedWorkdir string
	Runner          Runner
}

// GuardResult stores verified attributes returned by the VM probe.
type GuardResult struct {
	User    string
	Workdir string
}

// DefaultGuardOptions returns default isolation verification options.
func DefaultGuardOptions() GuardOptions {
	return GuardOptions{
		ExpectedUser: "boite",
		Runner:       NewDefaultRunner(),
	}
}

// BuildGuardProbeCommand generates the shell probe command executed in the target.
func BuildGuardProbeCommand(workdir string) string {
	cmd := "whoami; pwd"
	if workdir != "" {
		quoted := quoteArg(workdir)
		cmd += fmt.Sprintf("; mkdir -p %s 2>/dev/null; if [ -d %s ]; then echo WORKDIR_OK; else echo WORKDIR_MISSING; fi", quoted, quoted)
	}
	return cmd
}

// ParseGuardOutput parses the stdout lines produced by the probe command.
func ParseGuardOutput(output string) (*GuardResult, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("insufficient probe output: %q", output)
	}
	res := &GuardResult{
		User:    strings.TrimSpace(lines[0]),
		Workdir: strings.TrimSpace(lines[1]),
	}
	return res, nil
}

// VerifyIsolation ensures that user identity and directory match expectations.
//
// Root is not refused. Connecting as root to a host you own is a deliberate
// choice, and it is the user's to make: bulle runs their tools on their machine.
// The expected-user check is the real floor, because it pins the login to the
// user the target actually asked for — a boite VM that silently answered as
// root still fails, since its target names `boite`.
func VerifyIsolation(res *GuardResult, rawOut string, opts GuardOptions) error {
	if res == nil {
		return errors.New("guard result is nil")
	}
	if opts.ExpectedUser != "" && res.User != opts.ExpectedUser {
		return fmt.Errorf("isolation guard failed: target user %q does not match expected %q", res.User, opts.ExpectedUser)
	}
	if opts.ExpectedWorkdir != "" && !strings.Contains(rawOut, "WORKDIR_OK") {
		return fmt.Errorf("isolation guard failed: project directory %q does not exist in target", opts.ExpectedWorkdir)
	}
	return nil
}

func runProbe(ctx context.Context, target *Target, user string, cmd string, runner Runner) ([]byte, error) {
	args := buildRemoteSSHArgs(target, user, cmd, "")
	return runner.Run(ctx, "ssh", args...)
}

// PreflightCheck verifies target state, connectivity, user isolation, and workspace directory.
func PreflightCheck(ctx context.Context, target *Target, opts GuardOptions) (*GuardResult, error) {
	if target == nil {
		return nil, errors.New("target is nil")
	}
	if target.Backend == "boite" && target.Status != "running" {
		return nil, fmt.Errorf("sandbox %q is %s, must be running (try: boite start %s)", target.Name, target.Status, target.Name)
	}
	runner := opts.Runner
	if runner == nil {
		runner = NewDefaultRunner()
	}
	user := opts.ExpectedUser
	if user == "" {
		user = target.User
	}
	if user == "" && target.Backend == "boite" {
		user = "boite"
	}
	probeCmd := BuildGuardProbeCommand(opts.ExpectedWorkdir)
	out, err := runProbe(ctx, target, user, probeCmd, runner)
	if err != nil {
		return nil, probeError(target, out, err)
	}
	res, err := ParseGuardOutput(string(out))
	if err != nil {
		return nil, err
	}
	if err := VerifyIsolation(res, string(out), opts); err != nil {
		return nil, err
	}
	return res, nil
}
