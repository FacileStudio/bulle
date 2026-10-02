package sandbox

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/FacileStudio/bulle/internal/settings"
)

// sshEndpoint is the host, identity and workspace of an SSH target before it
// is named. It exists so the six values reach one constructor instead of a
// six-argument function.
type sshEndpoint struct {
	host    string
	port    int
	user    string
	keyPath string
	root    string
}

// targetFromSSH creates a Target configured for a generic SSH host. An unset
// port stays zero rather than defaulting to 22, so buildRemoteSSHArgs omits -p
// and lets ssh resolve the port from ~/.ssh/config or its own default.
func targetFromSSH(name string, ep sshEndpoint) *Target {
	h := ep.host
	if h == "" {
		h = "127.0.0.1"
	}
	return &Target{
		Name:    name,
		Backend: "ssh",
		Host:    h,
		Port:    ep.port,
		User:    ep.user,
		KeyPath: ep.keyPath,
		Workdir: ep.root,
		Status:  "running",
	}
}

// parseSSHAddress splits user@host:port into its parts. A bare host leaves the
// port at zero and the user empty, so the caller falls back to remote.port and
// the user's own ssh_config rather than a port guessed here.
func parseSSHAddress(addr string) (user, host string, port int) {
	remaining := addr
	if at := strings.Index(remaining, "@"); at != -1 {
		user = remaining[:at]
		remaining = remaining[at+1:]
	}
	if h, pStr, err := net.SplitHostPort(remaining); err == nil {
		host = h
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	} else {
		host = remaining
	}
	return user, host, port
}

func remoteTargetFromConfig(name string, t settings.RemoteTarget, cfg settings.Config) *Target {
	return targetFromSSH(name, sshEndpoint{
		host:    cmp.Or(t.Host, name),
		port:    cmp.Or(t.Port, cfg.Remote.Port),
		user:    cmp.Or(t.User, cfg.Remote.User),
		keyPath: cmp.Or(t.SSHKeyPath, cfg.Remote.SSHKeyPath),
		root:    cmp.Or(t.Root, cfg.Remote.Root),
	})
}

func resolveRemoteFromConfig(name string, cfg settings.Config) (*Target, bool) {
	t, ok := cfg.Remote.Targets[name]
	if !ok {
		return nil, false
	}
	return remoteTargetFromConfig(name, t, cfg), true
}

func resolveRemoteFromAddress(name string, cfg settings.Config) *Target {
	user, host, port := parseSSHAddress(name)
	return targetFromSSH(name, sshEndpoint{
		host:    host,
		port:    cmp.Or(port, cfg.Remote.Port),
		user:    cmp.Or(user, cfg.Remote.User),
		keyPath: cfg.Remote.SSHKeyPath,
		root:    cfg.Remote.Root,
	})
}

// ResolveRemoteTarget resolves an SSH host: a remote.targets entry first, then
// a direct user@host:port address or an ssh_config alias, both of which ssh
// itself understands.
func ResolveRemoteTarget(name string, cfg settings.Config) (*Target, error) {
	targetName := cmp.Or(name, cfg.Remote.Default)
	if targetName == "" {
		return nil, errors.New("remote target is required: pass a host or user@host:port, or set remote.default in settings")
	}
	if target, found := resolveRemoteFromConfig(targetName, cfg); found {
		return target, nil
	}
	target := resolveRemoteFromAddress(targetName, cfg)
	if target.Host == "" {
		return nil, fmt.Errorf("remote target %q has no host", targetName)
	}
	return target, nil
}
