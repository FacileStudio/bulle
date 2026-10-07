package tui

import "github.com/FacileStudio/bulle/internal/compaction"

const (
	// compactMinResult and droppedNotice alias the compaction package's tombstone
	// shape, so the report and the tests read the same strings the pass writes.
	compactMinResult = compaction.MinResult
	droppedNotice    = compaction.DroppedNotice
)

// estTokens delegates to the package's bytes-to-tokens estimate, kept as a name
// because the whole TUI reads it.
func estTokens(bytes int) int64 {
	return compaction.EstTokens(bytes)
}

// maskHistory runs the deterministic tombstone over the plan's history spans and
// debits the session size by the byte estimate. The debit is deliberately an
// estimate: the next sized() overwrites it with the backend's own count, so the
// two never drift past the turn boundary.
//
// Only tool results are counted, because only tool results are what the pass
// takes out: reasoning is never sent to a backend, so a stub on it would move
// this number without moving the context it stands for.
func (m *Model) maskHistory(plan []compaction.Span) compaction.MicroStats {
	stats := compaction.Tombstone(m.conversation, plan)
	if stats.Bytes == 0 {
		return stats
	}
	m.trimmed += stats.Results
	m.size = max(m.size-estTokens(stats.Bytes), 0)
	return stats
}

// applyMaskFallback runs the tombstone in place on the UI thread for the two
// cases where the summary did not happen: the summarizer errored, or produced
// nothing. The mask still frees the bulky tool output, so a failed summary costs
// nothing but the attempt.
//
// It reports whether it masked, and it refuses a plan that no longer covers the
// conversation: a pass measured against a conversation that has since been
// replaced has no history spans to hand here, and tombstoning whatever sits at
// those indices now would be an edit to a conversation the pass never measured.
// The caller is what turns the false into a notice that says so.
func (m *Model) applyMaskFallback(outcome compaction.Outcome) bool {
	if !compaction.Covers(m.conversation, outcome.Plan) {
		return false
	}
	before := m.size
	stats := m.maskHistory(outcome.Plan)
	start, end, _ := compaction.HistoryRange(outcome.Plan)
	report := compaction.Outcome{
		Before: before,
		After:  m.size,
		Plan:   outcome.Plan,
		Tier:   outcome.Tier,
	}
	m.last = compacted{
		evictCut: end - start,
		kept:     len(m.conversation) - end,
		results:  stats.Results,
		tier:     outcome.Tier,
	}
	m.say(fromCompact, compactReport(report, m.last))
	return true
}

// maskNote is what the mask managed, appended to a notice that had to mention
// it: a fallback that did not land must not claim it did, so the one line that
// says "masked instead" is the one place that has to ask.
func maskNote(masked bool) string {
	if masked {
		return " — masked instead"
	}
	return " — nothing was masked: the conversation changed while the pass ran"
}
