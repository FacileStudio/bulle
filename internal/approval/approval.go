// Package approval manages interactive and non-interactive tool call authorization.
package approval

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/toolview"
	"github.com/FacileStudio/nacelle"
)

// Decision is the answer to a pending tool approval request.
type Decision int

const (
	Denied Decision = iota
	AllowedOnce
	AllowedForSession
)

// Request represents a tool call approval request.
type Request struct {
	Name     string
	Input    json.RawMessage
	Decision chan<- Decision
}

// Approvals coordinates interactive tool authorization across goroutines.
type Approvals struct {
	asking  sync.Mutex
	mu      sync.Mutex
	allowed map[string]bool
	send    func(tea.Msg)
}

// New creates a new approval gate.
func New() *Approvals {
	return &Approvals{allowed: make(map[string]bool)}
}

// Wire connects the gate to the running program message delivery.
func (a *Approvals) Wire(send func(tea.Msg)) {
	if a != nil {
		a.send = send
	}
}

// IsAllowed reports whether tool name has been approved for the session.
func (a *Approvals) IsAllowed(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allowed[name]
}

// Allow marks a tool as approved for the entire session.
func (a *Approvals) Allow(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowed[name] = true
}

// Ask requests approval for a tool call, blocking until decided or cancelled.
func (a *Approvals) Ask(ctx context.Context, name string, input json.RawMessage) bool {
	if _, err := toolview.StrictObject(input); errors.Is(err, toolview.ErrDuplicateKey) {
		return false
	}
	if name == "tasks" {
		return true
	}
	if a.IsAllowed(name) {
		return true
	}
	a.asking.Lock()
	defer a.asking.Unlock()
	if a.IsAllowed(name) {
		return true
	}
	if a.send == nil {
		return false
	}
	decision := make(chan Decision, 1)
	a.send(Request{Name: name, Input: input, Decision: decision})
	select {
	case d := <-decision:
		if d == AllowedForSession {
			a.Allow(name)
		}
		return d != Denied
	case <-ctx.Done():
		return false
	}
}

// Accept passes legible input without prompting, refusing only ambiguous input.
func (a *Approvals) Accept(_ context.Context, name string, input json.RawMessage) bool {
	if _, err := toolview.StrictObject(input); errors.Is(err, toolview.ErrDuplicateKey) {
		return false
	}
	return true
}

// Build constructs the approval gate and returns the approval function.
func Build(approveTools bool) (*Approvals, nacelle.Approve) {
	gate := New()
	if approveTools {
		return gate, gate.Ask
	}
	return nil, gate.Accept
}
