package compaction

import "github.com/FacileStudio/nacelle"

// Outcome holds the state and result of a compaction pass.
type Outcome struct {
	Before      int64
	After       int64
	Plan        []Span
	Fold        Fold
	Tier        Tier
	Summary     string
	Judged      bool
	Consolidate bool
	Stage       string
	Err         error
}

// FailedAt names which stage of a compaction pass failed.
func (o Outcome) FailedAt() string {
	if o.Stage != "" {
		return o.Stage
	}
	return "summary"
}

// Installs reports whether a finished pass may replace the conversation.
func (o Outcome) Installs() bool {
	return o.Summary != "" || (o.Judged && o.Fold.LedgerSize() == 0)
}

// Freed reports the token count reduction between before and after sizes.
func (o Outcome) Freed() int64 {
	if o.After <= 0 || o.Before <= o.After {
		return 0
	}
	return o.Before - o.After
}

// Apply reassembles the conversation using the outcome and records after size.
func (o *Outcome) Apply(conv []nacelle.Message) ([]nacelle.Message, Stats) {
	newConv, stats := Apply(conv, o.Plan, o.Summary, o.Fold.Survives, o.Consolidate)
	if !stats.Stale && !stats.Refused {
		o.After = max(o.Before-stats.Before+stats.After, 0)
	} else {
		o.After = o.Before
	}
	return newConv, stats
}
