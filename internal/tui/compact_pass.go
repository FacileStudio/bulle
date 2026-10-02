// Package tui — the snapshot one compaction pass runs on.
package tui

import (
	"slices"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// compactPass is everything one pass needs, snapshotted on the update loop
// before the pass's goroutine starts. A pass is the one thing in this package
// that reads a conversation off the update loop's goroutine, so it is handed its
// own view instead of reaching into the model: a conversation the reader's next
// command cannot shorten under it, and a size the next sized() cannot move.
//
// That keeps the read safety local to this struct. It used to be an invariant
// spread across every command that might mutate the session mid-pass — ask
// queues while compacting, the pre-send path leaves the run busy — which held but
// was invisible at the point it mattered. What is left for the runtime to catch is
// the case the invariant never covered: the live conversation really did move on,
// and Apply's Covers check is what refuses a plan measured against it.
type compactPass struct {
	conv  []nacelle.Message
	size  int64
	plan  []compaction.Span
	tier  compaction.Tier
	judge compaction.Judge
	agent *nacelle.Agent
	// consolidate is whether this pass may rewrite the ledger rather than add
	// to it, decided here on the update loop from the ledger the conversation
	// already carries. It is the pass's question and not the assembler's because
	// it is answered by a size the goroutine cannot measure: the ledger lives in
	// the conversation, which the pass snapshotted but must not re-derive from.
	// It also decides the fold: a consolidating pass folds the history rather
	// than trusting the judge's keeps, so the rewrite it asks for has turns to
	// be measured against — see runCompaction.
	consolidate bool
	// force is overflow recovery's lever: a run the provider refused for length
	// has already proven the ladder's estimate wrong, so the pass folds the whole
	// history outright rather than asking whether the gentle fold would land.
	force bool
	// trigger is the size the pass is trying to land under, read from the policy
	// here because the pass goroutine must not touch the model. It is what
	// LandsUnder measures a fold against.
	trigger int64
}

// pass snapshots the session for one pass. The snapshot owns the conversation it
// hands the goroutine: the slice is cloned, and so is every message's Parts
// array. The second clone is the one that matters. Tombstone — the only thing in
// this package that edits a message in place — writes a stub into a result's slot
// rather than replacing the array, so a snapshot sharing those arrays would be
// reading words the mask was rewriting under it, on the update loop, at any
// moment the two paths overlapped.
//
// That sharing was the whole hazard, and cloning the parts is what makes it
// unrepresentable rather than merely unobserved. It is cheap in the unit that
// counts here: a parts array copies as a slice of interface headers, so the price
// is one small allocation per message and never the bytes those parts point at —
// measured at ~20µs for a 400-message conversation carrying eight kilobytes a
// message (BenchmarkPassSnapshot), on the update loop, once per pass, next to a
// summarizer call measured in seconds.
//
// What isolation cannot do is keep a future caller honest: the mask still runs on
// the update loop, and no check here can see whether a pass is live — maskHistory
// is away from this file and its callers do not ask. compact_pass_test.go pins
// the shape instead: it snapshots a conversation, masks it the way such a caller
// would, and fails if the pass's own view moved.
//
// The summarizer is built here rather than in the goroutine for the same reason:
// it reads m.agent, and a pass must not touch the model at all.
func (m *Model) pass(plan []compaction.Span, tier compaction.Tier, force bool) compactPass {
	return compactPass{
		conv:        snapshot(m.conversation),
		size:        m.size,
		plan:        plan,
		tier:        tier,
		judge:       m.judge,
		agent:       m.summarizer(),
		consolidate: compaction.LedgerOverBudget(compaction.LedgerText(m.conversation, plan)),
		force:       force,
		trigger:     m.policy.Trigger(),
	}
}

// snapshot is the pass's own copy of a conversation: its own slice header, its
// own backing array, and its own Parts slice per message, so nothing the update
// loop edits in place can be read by the pass's goroutine. The parts ride as a
// copy of the slice, not of what they hold — every nacelle.Part is read-only, and
// the one editor here replaces an element of that slice rather than writing
// through it, which is why copying the slice is the whole of the protection.
func snapshot(conv []nacelle.Message) []nacelle.Message {
	out := slices.Clone(conv)
	for i := range out {
		out[i].Parts = slices.Clone(out[i].Parts)
	}
	return out
}
