package sandbox

import (
	"fmt"
	"strconv"
	"strings"
)

// resolveTargetHostPort returns the host to ssh to and the port to pass with
// -p, or zero when ssh should choose the port itself.
//
// An SSH host with no configured port keeps zero on purpose: passing -p 22
// would override the Port in ~/.ssh/config and send an alias defined there to
// the wrong port, while an explicit -p and ssh_config are not distinguishable
// once the port is flattened into the target. A boite VM has no ssh_config
// entry to consult, so it falls back to the boite port instead.
func resolveTargetHostPort(target *Target) (string, int) {
	host := "127.0.0.1"
	port := 0
	if target == nil {
		return host, port
	}
	if target.Host != "" {
		host = target.Host
	}
	if target.Port > 0 {
		port = target.Port
	} else if target.Backend == "boite" {
		port = 2226
	}
	return host, port
}

// workdirPrefix returns the cd prefix a remote command needs, or nothing when
// no workspace is configured: the command then runs in the SSH login directory.
func workdirPrefix(workDir string) string {
	if workDir == "" {
		return ""
	}
	return "cd " + quoteArg(workDir) + " && "
}

func resolveSSHUserDestination(target *Target, user, host string) string {
	u := user
	if u == "" && target != nil {
		u = target.User
	}
	if u != "" {
		return fmt.Sprintf("%s@%s", u, host)
	}
	return host
}

// buildRemoteSSHArgs builds the non-interactive ssh invocation every tool and
// probe shares. It disables agent forwarding and prompting everywhere, and
// turns on connection multiplexing when controlPath is set so the handshake is
// paid once per session instead of once per tool call.
//
// Host-key verification is relaxed for boite targets only: the microVM's key is
// generated per instance and never in known_hosts, and the connection is
// loopback. A remote host is a machine on a network, so it verifies against the
// user's own known_hosts and ssh_config exactly as a hand-typed ssh would; with
// BatchMode on, an unknown host is refused rather than silently trusted.
//
// A `--` ends option parsing before the destination, so a configured host value
// that happens to start with `-` is a hostname to ssh and not an option it
// would otherwise run.
func buildRemoteSSHArgs(target *Target, user, remoteCmd, controlPath string) []string {
	host, port := resolveTargetHostPort(target)
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ForwardAgent=no",
		"-o", "LogLevel=ERROR",
	}
	if target != nil && target.Backend == "boite" {
		args = append(args,
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
		)
	}
	if controlPath != "" {
		args = append(args,
			"-o", "ControlMaster=auto",
			"-o", "ControlPath="+controlPath,
			"-o", "ControlPersist=60s",
		)
	}
	if port > 0 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	if target != nil && target.KeyPath != "" {
		args = append(args, "-i", target.KeyPath)
	}
	args = append(args, "--", resolveSSHUserDestination(target, user, host))
	if remoteCmd != "" {
		args = append(args, remoteCmd)
	}
	return args
}

// isLoopbackHost reports whether a host is this machine, where a boite VM is the
// likely explanation for an untrusted key.
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// hostKeyHint recognizes the ssh host-key failures that otherwise surface as a
// bare exit status, and returns the command that resolves them.
//
// Every bulle ssh runs with BatchMode=yes, so ssh can never show the yes/no
// prompt a hand-typed connection would: an unknown key and a rotated one are
// both refused with nothing the user can act on. A boite VM never reaches here —
// its args turn verification off — so this is about network hosts, and the trust
// step stays explicit rather than automatic.
//
// `ssh-keyscan` reads no ssh_config, so it is only offered when the port is
// literal; an alias is left to `ssh`, which resolves it.
func hostKeyHint(target *Target, output string) (string, bool) {
	host, port := resolveTargetHostPort(target)
	ref := host
	if port > 0 {
		ref = fmt.Sprintf("[%s]:%d", host, port)
	}
	switch {
	case strings.Contains(output, "REMOTE HOST IDENTIFICATION HAS CHANGED"):
		return fmt.Sprintf("the host key for %s no longer matches ~/.ssh/known_hosts; remove the stale entry with `ssh-keygen -R %s` (ssh prints the exact entry when the config differs) and retry", ref, ref), true
	case strings.Contains(output, "Host key verification failed"):
		hint := fmt.Sprintf("the host key for %s is not in ~/.ssh/known_hosts; connect once with `ssh %s` to review and trust it", ref, sshConnectArgs(target, host, port))
		if port > 0 {
			hint += fmt.Sprintf(", or trust it non-interactively with `ssh-keyscan -p %d %s >> ~/.ssh/known_hosts`", port, host)
		}
		hint += ", then retry"
		if isLoopbackHost(host) {
			hint += "; for a local boite VM, use `bulle sandbox <vm>` instead"
		}
		return hint, true
	}
	return "", false
}

// sshConnectArgs is the tail of the hand-typed equivalent of the connection
// bulle tried: the port it would pass and the user it would log in as, with the
// ssh_config alias left for ssh itself to resolve.
func sshConnectArgs(target *Target, host string, port int) string {
	dest := resolveSSHUserDestination(target, "", host)
	if port <= 0 {
		return dest
	}
	return fmt.Sprintf("-p %d %s", port, dest)
}

// probeError turns a failed probe into the error a caller reports, preferring
// an actionable host-key hint over the raw exit status.
func probeError(target *Target, out []byte, err error) error {
	if hint, ok := hostKeyHint(target, string(out)); ok {
		return fmt.Errorf("preflight isolation probe failed on target %s: %s (%w)", target.Name, hint, err)
	}
	return fmt.Errorf("preflight isolation probe failed on target %s: %w (%s)", target.Name, err, strings.TrimSpace(string(out)))
}
