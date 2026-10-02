// Package ide publishes a running session to an editor over a unix socket,
// and carries the editor's answers back.
//
// Nothing here exists unless the session asked for it: with the surface off
// there is no socket, no discovery file and no goroutine, so a session nobody
// watches behaves exactly as it did before the feature existed. The publisher
// is an ordinary nacelle hook registered at the points a tool call is visible
// at, which is what keeps a second client of the session off the agent's own
// code path.
package ide

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// timeFormat is how a session's start is written down: the same instant in
// the same shape wherever it is read.
const timeFormat = time.RFC3339

// Options is what a session tells the socket about itself. Root is the tree
// the file tools may reach, and the one a published file change is resolved
// against.
type Options struct {
	Root    string
	Model   string
	Version string
	Session string
	// Commands is how an editor's prompts reach the session. Nil keeps the
	// socket an observer, which is how bulle runs today.
	Commands Commands
}

// Server is the session side of the IDE surface: the socket an editor dials,
// the file it finds that socket through, and the events it is sent.
type Server struct {
	opts    Options
	file    string
	path    string
	ln      *net.UnixListener
	started time.Time
	mu      sync.Mutex
	conns   []*conn
	waiting map[string]chan bool
	done    chan struct{}
	closed  bool
	wg      sync.WaitGroup
}

// Start opens the socket, publishes the discovery file and begins accepting
// editors. A failure here is returned rather than logged: a session that was
// asked to publish and cannot says so, instead of looking like one nobody is
// watching.
func Start(opts Options) (*Server, error) {
	pid := os.Getpid()
	opts.Root = absoluteRoot(opts.Root)
	path, err := SocketPath(pid)
	if err != nil {
		return nil, err
	}
	ln, err := listen(path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, errors.Join(err, ln.Close(), clearSocket(path))
	}
	s := &Server{
		opts: opts, file: File(pid), path: path, ln: ln, started: time.Now(),
		waiting: map[string]chan bool{}, done: make(chan struct{}),
	}
	if err := writeDiscovery(s.discovery(pid)); err != nil {
		return nil, errors.Join(err, ln.Close(), clearSocket(path))
	}
	setCurrent(s)
	s.wg.Go(s.accept)
	return s, nil
}

// absoluteRoot names the tree a published change is resolved against, as a
// path an editor can compare with its own working directory. The config's own
// root is often ".", which only means something next to this process.
func absoluteRoot(root string) string {
	if root == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

// Close drops every editor, refuses the calls still waiting for an answer,
// and removes the socket and the discovery file. It is idempotent, because a
// session reaches its end down more than one path, and a nil server is a
// process that publishes to nobody.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	if !s.shutdown() {
		return nil
	}
	stop := s.ln.Close()
	s.wg.Wait()
	return errors.Join(stop, clearSocket(s.path), removeDiscovery(s.file))
}

// shutdown marks the surface down exactly once and drops every editor, which
// is what lets the reads being served return. It reports whether this call was
// the one that did it.
func (s *Server) shutdown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.closed = true
	conns := s.conns
	s.conns = nil
	close(s.done)
	clearCurrent(s)
	for _, c := range conns {
		c.close()
	}
	return true
}

// SetCommands names the handler an attached editor's commands reach. It is set
// after the socket opens rather than in Options, because the handler is the
// session itself (the TUI's prompt loop) and the session does not exist yet
// when Start runs. Nil is accepted and turns the socket back into an observer.
func (s *Server) SetCommands(c Commands) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts.Commands = c
}

// Publish sends one event to every attached editor. A session with nobody
// attached pays a lock and a slice copy for it, and nothing else.
//
// A nil server is a session with the surface off, and publishing to it is
// silence rather than a panic: the hooks built from a nil server stay installed
// in a run either way, so the no-editor path must cost nothing and break
// nothing.
func (s *Server) Publish(ev Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	conns := slices.Clone(s.conns)
	s.mu.Unlock()
	for _, c := range conns {
		c.send(ev)
	}
}

// hello is the first line an editor reads: what it has attached to. Its
// caller holds mu, so a hello cannot be overtaken by an event.
func (s *Server) hello() Event {
	return Hello(os.Getpid(), s.opts.Root, s.opts.Session, s.opts.Model, s.opts.Version)
}

// setSession records the transcript path a session opened after its socket
// did, and rewrites the discovery file so a plugin reading that file is told
// what a connected editor already was. A nil server is a process that
// publishes to nobody, and has nothing to record.
//
// The rewrite happens under the lock the close takes, because a write that
// landed after the close would leave a discovery file naming a socket that is
// already gone.
func (s *Server) setSession(path string) error {
	if s == nil || path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.opts.Session = path
	return writeDiscovery(s.fields(os.Getpid()))
}
