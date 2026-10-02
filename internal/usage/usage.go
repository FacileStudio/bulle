// Package usage records session metrics to disk.
package usage

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/nacelle"
)

type canonicalEvent struct {
	Type      string         `json:"type"`
	Role      string         `json:"role"`
	Timestamp string         `json:"timestamp"`
	Agent     string         `json:"agent"`
	Machine   string         `json:"machine"`
	Project   string         `json:"project"`
	Branch    string         `json:"branch,omitempty"`
	Model     string         `json:"model,omitempty"`
	Usage     canonicalUsage `json:"usage"`
}

type canonicalUsage struct {
	Input      int64          `json:"input"`
	Output     int64          `json:"output"`
	CacheRead  int64          `json:"cacheRead"`
	CacheWrite int64          `json:"cacheWrite"`
	Cost       *canonicalCost `json:"cost,omitempty"`
}

type canonicalCost struct {
	Total float64 `json:"total"`
}

// Sink appends one canonical event per finished turn to mycelium's event feed.
type Sink struct {
	dir     string
	machine string
	project string
	branch  string
	model   string
}

// NewSink returns a sink when mycelium's data directory exists, or nil when it is absent.
func NewSink(root, model string) *Sink {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		dataDir = filepath.Join(home, ".mycelium")
	}
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		return nil
	}
	machine, _ := os.Hostname()
	project, branch := repoIdentity(root)
	return &Sink{
		dir:     filepath.Join(dataDir, "events", "bulle"),
		machine: machine,
		project: project,
		branch:  branch,
		model:   model,
	}
}

// Record appends one turn's usage to the event log.
func (s *Sink) Record(usage nacelle.Usage, now time.Time) {
	if s == nil {
		return
	}
	line, err := json.Marshal(s.canonical(usage, now))
	if err != nil {
		return
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(s.dir, now.UTC().Format("2006-01")+".jsonl")
	if err := appendLine(path, line); err != nil {
		return
	}
}

func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, written := f.Write(append(line, '\n'))
	return errors.Join(written, f.Close())
}

func (s *Sink) canonical(usage nacelle.Usage, now time.Time) canonicalEvent {
	event := canonicalEvent{
		Type:      "message",
		Role:      "assistant",
		Timestamp: now.UTC().Format(time.RFC3339Nano),
		Agent:     "bulle",
		Machine:   s.machine,
		Project:   s.project,
		Branch:    s.branch,
		Model:     s.model,
		Usage: canonicalUsage{
			Input:      usage.InputTokens,
			Output:     usage.OutputTokens,
			CacheRead:  usage.CacheReadTokens,
			CacheWrite: usage.CacheCreationTokens,
		},
	}
	if usage.Cost > 0 {
		event.Usage.Cost = &canonicalCost{Total: usage.Cost}
	}
	return event
}

// repoIdentity names the project a run belongs to and the branch it sat on. A
// target-labeled root names a remote machine, not a repo here: running host git
// against it would silently report the project bulle was launched from, so the
// target's own name is the identity and there is no branch to name.
func repoIdentity(root string) (string, string) {
	if name := settings.TargetRootName(root); name != "" {
		return name, ""
	}
	if root == "" {
		root = "."
	}
	project := filepath.Base(root)
	if abs, err := filepath.Abs(root); err == nil {
		project = filepath.Base(abs)
	}
	if out, err := gitIn(root, "rev-parse", "--show-toplevel"); err == nil && out != "" {
		project = filepath.Base(out)
	}
	if out, err := gitIn(root, "remote", "get-url", "origin"); err == nil {
		if name := repoNameFromRemote(out); name != "" {
			project = name
		}
	}
	branch, err := gitIn(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || branch == "HEAD" {
		branch = ""
	}
	return project, branch
}

func gitIn(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func repoNameFromRemote(url string) string {
	url = strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	if url == "" || url == "." || url == ".." || strings.ContainsAny(url, " \\") {
		return ""
	}
	return url
}
