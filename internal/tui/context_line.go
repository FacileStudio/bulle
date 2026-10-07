package tui

import (
	"fmt"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/cost"
	"github.com/FacileStudio/bulle/internal/status"
)

// contextLoad is the status surface's name for the conversation's size against
// the window it is measured on: `↕120k/160k · 0.75` once the backend reports a
// window, a plain `↕120k` when it does not, with the tier appended once the size
// has crossed one. One shape, so the footer and /status cannot disagree about
// how full the context is.
//
// The denominator is the window a turn can actually fill — the backend's window
// less the reserve held back for the answer — because that is the figure the
// ladder is read against. Showing the raw window instead would print a ratio
// below smart_ratio while the session was already compacting at smart, which is
// exactly the kind of disagreement the tier suffix exists to prevent; the raw
// window and the reserve are named on their own line in /status.
func contextLoad(size int64, policy compaction.Policy) string {
	load := "↕" + status.ShortTokens(size)
	if usable := policy.Usable(); usable > 0 {
		load += "/" + status.ShortTokens(usable)
		load += fmt.Sprintf(" · %.2f", float64(size)/float64(usable))
	}
	if tier := policy.Tier(size); tier != compaction.Below {
		load += " · " + tier.String()
	}
	return load
}

// compactAtLine names the figure the ladder switches on, which is the one number
// /status was missing: the footer reads the size against the window it can fill
// and appends the tier the size has reached, so a session whose compact_at sits
// under the soft ratio prints a share below 0.65 under a `soft` suffix, and
// nothing on screen said which number put the tier there. Naming it against the
// same denominator as the footer is what makes the two reconcilable — 50.0k of
// 160k usable is 0.31, so the footer's 0.38 is over it.
//
// The ratio is left off when there is no window to measure it against, the way
// contextLoad leaves off its own: a windowless backend has no usable figure for
// either line to be a share of.
func compactAtLine(compactAt int64, policy compaction.Policy) string {
	if compactAt <= 0 {
		return ""
	}
	line := "compact at · " + status.ShortTokens(compactAt)
	if usable := policy.Usable(); usable > 0 {
		line += " · " + fmt.Sprintf("%.2f", float64(compactAt)/float64(usable)) + " of " + status.ShortTokens(usable) + " usable"
	}
	return line
}

// compactionLines is what /status adds about the ladder beyond the footer's own
// figure: the size against the window with the tier it has reached, the compact_at
// the tier is read against, the window itself with the reserve the ladder holds
// back from it, the accumulated ledger with the tier of the last pass that wrote
// it, and the judge when it is on. A session with nothing to say — no size
// measured, no window, no ledger yet, no judge — adds no lines, and the trigger
// is the one exception: it is a setting like the judge rather than a measurement,
// so a session that has compacted nothing yet still names where it would start.
//
// The judge gets a line of its own because it is the one setting that sends the
// conversation off the machine, and it is otherwise invisible once enabled: a
// reader who opted in months ago has nothing to remind them that every pass
// ships the history somewhere. It sits with the ladder because it is part of it —
// the judge is what decides which turns the fold is allowed to take.
//
// A second line names the model version that actually answered and what it billed,
// once one has. The setting defaults to the vendor's drifting `jev-latest` alias,
// and the thresholds are tuned against a specific version behind it, so the id has
// to be readable before it can be pinned.
func (m *Model) compactionLines() []string {
	policy := m.engine().Policy
	var lines []string
	if m.size > 0 {
		lines = append(lines, contextLoad(m.size, policy))
	}
	if line := compactAtLine(m.compactAt, policy); line != "" {
		lines = append(lines, line)
	}
	if window := policy.Window; window > 0 {
		lines = append(lines, "window · "+status.ShortTokens(window)+" raw, "+status.ShortTokens(policy.Usable())+
			" usable, "+status.ShortTokens(policy.Reserve)+" reserved for the answer")
	}
	if ledger := compaction.LedgerText(m.conversation, m.plan()); ledger != "" {
		line := "ledger · ~" + status.ShortTokens(compaction.EstTokens(len(ledger))) + " tokens"
		if m.last.tier != compaction.Below {
			line += " · last pass " + m.last.tier.String()
		}
		lines = append(lines, line)
	}
	if judge := m.engine().Judge; judge != nil {
		lines = append(lines, "judge · on — each pass sends the history off the machine")
		if answer := lastAnswer(judge); answer.Model != "" {
			lines = append(lines, judgeModelLine(answer))
		}
	}
	return lines
}

// judgeModelLine names the version that answered and what it billed, so a reader
// can pin limits.compaction.judge.model to it instead of leaving the alias in
// place. The bill is left off when the vendor reported none.
func judgeModelLine(answer compaction.Answer) string {
	line := "judge model · " + answer.Model
	if answer.Provider != "" {
		line += " via " + answer.Provider
	}
	if answer.InputTokens > 0 {
		line += " · " + status.ShortTokens(int64(answer.InputTokens)) + " tokens in"
	}
	if answer.CostUSD > 0 {
		line += " · " + cost.FormatCost(answer.CostUSD)
	}
	return line
}

// lastAnswer reads the model version and the bill of a judge's most recent call
// when the judge reports them. A judge backed by a versioned remote service
// implements compaction.Reporter; one that decides locally — a test's stub — has
// no answer to give and contributes no line.
func lastAnswer(judge compaction.Judge) compaction.Answer {
	if reporter, ok := judge.(compaction.Reporter); ok {
		return reporter.LastAnswer()
	}
	return compaction.Answer{}
}
