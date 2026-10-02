// Package herdr reports this pane's agent state to a running herdr
// multiplexer over its socket API. bulle is not one of the agent kinds
// compiled into herdr, so herdr would show the pane as a plain shell; a
// process whose name herdr does not know can still claim a label through
// `pane report-agent`, and herdr treats that report as the sole authority.
// Reporting here is what makes a bulle session show up in herdr at all.
package herdr

import (
	"os"
	"os/exec"
	"sync"
)

const (
	Idle    = "idle"
	Working = "working"
	Blocked = "blocked"
)

// source is the stable, unique source id every report from this process uses.
// Herdr keys lifecycle authority by (source, agent); keeping both fixed here
// means the reports never compete and never get released by anything else.
const source = "bulle"

// Client reports to one pane. spawn is injected so tests can record the
// commands that would run instead of launching a real herdr; production wires
// it to the binary's own CLI.
type Client struct {
	bin   string
	pane  string
	state string
	mu    sync.Mutex
	spawn func(args []string) error
	// session is the on-disk path of this run's transcript, reported on every
	// transition so herdr holds the pane's session identity — the same the
	// built-in Crush and Prime integrations report. It is set once the session
	// opens, so the startup idle report (issued before Launch opens one) goes
	// out without it and the first working report carries it.
	session string
}

// SetSession records the transcript path this run writes, so later reports
// carry a session reference herdr can store on the pane.
func SetSession(h *Client, path string) {
	if h == nil || path == "" {
		return
	}
	h.session = path
}

// NewFromEnv builds a client from the environment herdr injects into every
// pane, returning nil when bulle is not running inside herdr. Guarding on
// HERDR_ENV makes the integration a no-op outside herdr, and refusing to build
// when the pane or binary is missing keeps a partial environment from sending
// reports into nothing.
func NewFromEnv() *Client {
	if os.Getenv("HERDR_ENV") != "1" {
		return nil
	}
	bin := os.Getenv("HERDR_BIN_PATH")
	pane := os.Getenv("HERDR_PANE_ID")
	if bin == "" || pane == "" {
		return nil
	}
	return &Client{
		bin:  bin,
		pane: pane,
		spawn: func(args []string) error {
			cmd := &exec.Cmd{Path: args[0], Args: args}
			return cmd.Run()
		},
	}
}

// Report claims the pane's agent state, sending only when it changes so a
// long run does not fire one subprocess per streamed event. Reports are
// serialised behind a mutex and awaited before the next one is launched,
// which is what guarantees herdr reads them in the order they were issued;
// without the wait, two quick transitions could race onto the socket and
// land out of order. A state only counts as sent once the report succeeds,
// so a pane herdr answers back quickly and a failed report is retried the
// next time a transition reaches for the same state.
func Report(h *Client, state string) {
	if h == nil || h.state == state {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state == state {
		return
	}
	args := []string{
		h.bin, "pane", "report-agent", h.pane,
		"--source", source, "--agent", source, "--state", state,
	}
	if h.session != "" {
		args = append(args, "--agent-session-path", h.session)
	}
	if err := h.spawn(args); err != nil {
		return
	}
	h.state = state
}

// Release drops the agent's lifecycle authority when bulle exits, so herdr
// does not keep showing a dead pane as bulle. It runs before the process
// ends, so the awaited subprocess completes in time.
func Release(h *Client) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.spawn([]string{
		h.bin, "pane", "release-agent", h.pane,
		"--source", source, "--agent", source,
	}); err != nil {
		return
	}
}
