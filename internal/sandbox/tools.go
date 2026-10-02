package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// ToolsOptions configures remote SSH-backed tool execution.
type ToolsOptions struct {
	Target         *Target
	WorkDir        string
	Runner         Runner
	CommandTimeout time.Duration
	MaxOutputBytes int
	MaxReadBytes   int
	// ControlPath is the ssh ControlMaster socket shared by every tool call.
	// RemoteTools fills it in; a session built by hand without one pays a
	// handshake per call.
	ControlPath string
}

type remoteSession struct {
	opts ToolsOptions
}

type remoteCloser struct {
	cleanup func() error
}

func (c *remoteCloser) Close() error {
	if c.cleanup != nil {
		return c.cleanup()
	}
	return nil
}

// quoteArg makes one argument safe for the remote POSIX shell. Single quotes
// are the one form a shell never looks inside: unlike double quotes, they stop
// `$` and backticks from expanding, which matters for a path the model chose.
//
// A leading `~` is expanded through "$HOME" rather than quoted away, so a
// home-relative path still means the remote user's home — the reason
// resolveRemotePath leaves `~` alone in the first place.
func quoteArg(p string) string {
	if p == "~" {
		return `"$HOME"`
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return `"$HOME"/` + shellQuote(rest)
	}
	return shellQuote(p)
}

// shellQuote single-quotes p when the shell would read anything in it, and
// leaves it bare otherwise so an ordinary path stays readable in a probe.
func shellQuote(p string) string {
	if p == "" {
		return "''"
	}
	if !strings.ContainsAny(p, " \t\n\"'$`\\*?[]()~;&|<>") {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// resolveRemotePath resolves a tool path against the target workspace. An
// absolute or home-relative path is left alone; a relative path with no
// configured workspace stays relative, so the remote shell resolves it against
// the SSH login directory.
func resolveRemotePath(workDir, p string) string {
	trimmed := strings.TrimSpace(p)
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~") {
		return path.Clean(trimmed)
	}
	if workDir == "" {
		return path.Clean(trimmed)
	}
	return path.Clean(path.Join(workDir, trimmed))
}

func (s *remoteSession) runSSH(ctx context.Context, remoteCmd string) ([]byte, error) {
	runner := s.opts.Runner
	user := ""
	if s.opts.Target != nil {
		user = s.opts.Target.User
	}
	args := buildRemoteSSHArgs(s.opts.Target, user, remoteCmd, s.opts.ControlPath)
	return runner.Run(ctx, "ssh", args...)
}

// newRemoteSession fills in the timeouts and limits a session needs. WorkDir is
// deliberately left as given: empty means the SSH login directory, which is
// where a bare ssh lands, rather than a path invented here.
func newRemoteSession(opts ToolsOptions) *remoteSession {
	resolved := opts
	if resolved.CommandTimeout == 0 {
		resolved.CommandTimeout = 2 * time.Minute
	}
	if resolved.MaxOutputBytes == 0 {
		resolved.MaxOutputBytes = 64 * 1024
	}
	if resolved.MaxReadBytes == 0 {
		resolved.MaxReadBytes = 48 * 1024
	}
	if resolved.Runner == nil {
		resolved.Runner = NewDefaultRunner()
	}
	return &remoteSession{opts: resolved}
}

func buildTools(s *remoteSession) ([]nacelle.Tool, error) {
	builders := []func(*remoteSession) (nacelle.Tool, error){
		buildCommandTool,
		buildReadTool,
		buildWriteTool,
		buildEditTool,
		buildListTool,
		buildFindTool,
		buildSearchTool,
	}
	tools := make([]nacelle.Tool, 0, len(builders))
	for _, build := range builders {
		tool, err := build(s)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// RemoteTools constructs the remote tool set and a closer for the ssh
// ControlMaster socket directory the tools multiplex over. The caller owns the
// closer and must close it when the session ends.
func RemoteTools(opts ToolsOptions) ([]nacelle.Tool, io.Closer, error) {
	s := newRemoteSession(opts)
	socketDir, err := os.MkdirTemp("", "bulle-ssh-")
	if err != nil {
		return nil, nil, fmt.Errorf("creating ssh control socket directory: %w", err)
	}
	s.opts.ControlPath = filepath.Join(socketDir, "ctrl")
	tools, err := buildTools(s)
	if err != nil {
		return nil, nil, errors.Join(err, os.RemoveAll(socketDir))
	}
	return tools, &remoteCloser{cleanup: func() error { return os.RemoveAll(socketDir) }}, nil
}
