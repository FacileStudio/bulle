package ide

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Protocol is the version every discovery file and every object on the wire
// carries. A receiver that meets another one refuses it rather than guessing
// at the meaning of fields it was never told about.
const Protocol = 1

// Discovery is the file one running session leaves for an editor to find it
// by: which process it is, where to dial, and which transcript it writes.
type Discovery struct {
	V       int    `json:"v"`
	PID     int    `json:"pid"`
	Root    string `json:"root"`
	Socket  string `json:"socket"`
	Session string `json:"session"`
	Started string `json:"started"`
	Version string `json:"version"`
}

// Dir is where discovery files live, one per running process.
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".bulle", "ide")
}

// File is the path this process's discovery file takes.
func File(pid int) string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("%d.json", pid))
}

// discovery describes the running server as an editor needs to read it.
func (s *Server) discovery(pid int) Discovery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fields(pid)
}

// fields is discovery without the lock, for a caller that already holds it and
// has to keep holding it until the file is written.
func (s *Server) fields(pid int) Discovery {
	return Discovery{
		V:       Protocol,
		PID:     pid,
		Root:    s.opts.Root,
		Socket:  s.path,
		Session: s.opts.Session,
		Started: s.started.UTC().Format(timeFormat),
		Version: s.opts.Version,
	}
}

// writeDiscovery publishes one file, 0600 inside a 0700 directory: the file
// names a socket that answers a session's tool calls, so no other user on the
// machine gets to read it, replace it, or dial the socket it points at.
func writeDiscovery(d Discovery) error {
	if Dir() == "" {
		return errors.New("no home directory to write the IDE discovery file in")
	}
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	path := File(d.PID)
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// removeDiscovery takes this process's file back down. A file left behind by
// a process that was killed is the reader's problem rather than ours: a
// reader checks that the pid is alive before dialling what it finds.
func removeDiscovery(path string) error {
	if path == "" {
		return nil
	}
	return clearSocket(path)
}
