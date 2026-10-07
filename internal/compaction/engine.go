package compaction

import (
	"time"

	"github.com/FacileStudio/nacelle"
)

// Default limits and timeouts for compaction passes.
const (
	DefaultJudgeTimeout     = 30 * time.Second
	DefaultSummarizeTimeout = 120 * time.Second
	DefaultThrashLimit      = 3
)

// AgentBuilder constructs a summarizer agent.
type AgentBuilder func() (*nacelle.Agent, error)

// Engine coordinates context compaction passes and guards against thrashing.
type Engine struct {
	Policy           Policy
	Judge            Judge
	Builder          AgentBuilder
	JudgeTimeout     time.Duration
	SummarizeTimeout time.Duration
	ThrashLimit      int
	thrashCount      int
}

// NewEngine creates an Engine with the provided policy, judge, and agent builder.
func NewEngine(policy Policy, judge Judge, builder AgentBuilder) *Engine {
	return &Engine{
		Policy:  policy,
		Judge:   judge,
		Builder: builder,
	}
}

// BackendAgent constructs an AgentBuilder from a nacelle.Backend.
func BackendAgent(backend nacelle.Backend) AgentBuilder {
	return func() (*nacelle.Agent, error) {
		if backend == nil {
			return nil, nil
		}
		return nacelle.New(nacelle.Config{
			Backend:       backend,
			System:        SystemPrompt,
			Thinking:      nacelle.Thinking{Show: false},
			MaxTokens:     MaxLedgerTokens,
			MaxIterations: 1,
		})
	}
}

// SetThrash sets the engine's thrash count to n.
func (e *Engine) SetThrash(n int) {
	e.thrashCount = n
}

// Thrashed reports whether the engine has reached its thrash limit.
func (e *Engine) Thrashed() bool {
	return e.thrashCount >= e.limit()
}

// ThrashCount returns the current count of consecutive unlanded passes.
func (e *Engine) ThrashCount() int {
	return e.thrashCount
}

// ResetThrash resets the thrash counter to zero.
func (e *Engine) ResetThrash() {
	e.thrashCount = 0
}

// CheckThrash records a pass result size and reports whether it is thrashed.
func (e *Engine) CheckThrash(size int64) bool {
	trigger := e.Policy.Trigger()
	if trigger > 0 && size > trigger {
		e.thrashCount++
		return e.thrashCount >= e.limit()
	}
	e.thrashCount = 0
	return false
}

func (e *Engine) limit() int {
	if e.ThrashLimit > 0 {
		return e.ThrashLimit
	}
	return DefaultThrashLimit
}
