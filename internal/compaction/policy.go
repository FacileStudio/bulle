// Package compaction owns bulle's context-management strategy: how a long
// conversation is partitioned into zones, which region is eligible for
// tombstoning or pruning, and how the surviving turns are reassembled around a
// persistent state ledger.
//
// It is pure. Nothing here owns a model, a clock or a backend, so the whole
// policy is table-driven from a test. The TypeSafe judge and the HTTP client
// that speaks to it live in their own packages; the TUI keeps only the wiring.
package compaction

import "github.com/FacileStudio/nacelle"

// The shipped tier ladder, the two ends a pass never touches, and the budget the
// verbatim tail is measured against. Settings alias these so the numbers cannot
// drift between the two layers.
//
// KeepTurns is a floor in messages and KeepTokens is the budget that does the
// sizing, and the floor is deliberately the smaller number. A message floor is
// the one bound that can pin a conversation open however much room the ladder
// has: the newest turns are exactly where a session's largest tool results land,
// and neither tier can touch a message the window is holding verbatim. So the
// shipped floor is one — the live turn alone, which no summary may stand in for
// — and forty thousand tokens of tail is the budget that sizes the rest.
const (
	DefaultSoftRatio      = 0.65
	DefaultSmartRatio     = 0.80
	DefaultKeepTurns      = 1
	DefaultKeepTokens     = 40_000
	DefaultAnchorMessages = 1
)

// Ratios is the tier ladder as fractions of the backend's context window.
// Soft tombstones history with no model call; smart adds a batched judge pass and
// a ledger summary, and forces the fold when the gentle one would not land.
//
// There were three rungs until the hard ratio was removed. It was a second
// threshold over one number standing in for a question that can be asked
// directly — does the fold just classified land under the trigger — so the
// answer is now derived in the caller from LandsUnder rather than read off a
// ratio. Every framework surveyed does unconditionally what that rung did, so
// nothing was lost by making it conditional instead of scheduled.
type Ratios struct{ Soft, Smart float64 }

// Tier is the rung a measured conversation size has reached.
type Tier uint8

const (
	// Below is under every ratio with nothing to do.
	Below Tier = iota
	// Soft is the deterministic, model-free tier.
	Soft
	// Smart adds the judge and the ledger summary, and forces the fold when the
	// gentle one does not land the conversation under the trigger.
	Smart
)

// String names a tier the way a status line or a report reads it.
func (t Tier) String() string {
	switch t {
	case Soft:
		return "soft"
	case Smart:
		return "smart"
	default:
		return "below"
	}
}

// Zone names the region one contiguous run of messages belongs to.
type Zone uint8

const (
	// ZoneAnchor is the pinned head: the first AnchorMessages messages, never
	// rewritten, summarized or pruned.
	ZoneAnchor Zone = iota
	// ZoneLedger is the single message carrying the state-ledger sentinel.
	ZoneLedger
	// ZoneHistory is everything between the ledger and the active window; the
	// only region eligible for tombstoning or pruning.
	ZoneHistory
	// ZoneActive is the newest KeepTurns messages, kept verbatim.
	ZoneActive
)

// Span is one contiguous run of a conversation: Zone says what it is, and
// Start and End are the half-open index range it covers.
type Span struct {
	Zone       Zone
	Start, End int
}

// Policy is everything one session needs to decide what to compact: the ratio
// ladder, the window it is measured against, the runway held back for the
// answer, the absolute ceiling that overrides the ladder, and the two ends a
// pass never touches.
//
// Window is the backend's whole context window and Reserve the part of it a turn
// needs to answer with; the ladder is measured against the difference between
// them, so the ratios describe a fraction of the window a session can actually
// fill — see Usable. Window is 0 when the backend reports none, and Ceiling is
// the compact_at fallback for exactly that case. KeepTurns is a floor in
// messages the tail never drops below, KeepTokens is the budget that sizes it
// beyond that floor, and AnchorMessages pins the head.
type Policy struct {
	Ratios         Ratios
	Window         int64
	Reserve        int64
	Ceiling        int64
	KeepTurns      int
	KeepTokens     int64
	AnchorMessages int
}

// Usable is the window a tier is measured against: the backend's own window less
// the runway a turn needs to finish. Reserving it is what stops the top rung
// from leaving the model nothing to answer with — a ratio read against the raw
// window leaves only its remainder for the response, where against the usable
// window the top rung leaves the reserve *and* that remainder, and on a
// model that reasons before it speaks the difference is a turn cut off mid-thought. Zero
// means there is no window to measure against, which is the windowless backend
// the absolute ceiling alone covers.
func (p Policy) Usable() int64 {
	if p.Window <= 0 {
		return 0
	}
	return max(p.Window-p.Reserve, 0)
}

// Tier is the rung size falls on. With a known window it is a ratio comparison
// against the window a turn can actually fill, with compact_at flooring it at
// the soft tier when a session pins one. With no window the ratios are
// undefined, so the ceiling is the only trigger and it buys the full pass: a
// backend that reports no window has no soft ratio to measure a free tombstone
// against.
func (p Policy) Tier(size int64) Tier {
	usable := p.Usable()
	if usable <= 0 {
		if p.Ceiling > 0 && size >= p.Ceiling {
			return Smart
		}
		return Below
	}
	switch {
	case reaches(size, p.Ratios.Smart, usable):
		return Smart
	case reaches(size, p.Ratios.Soft, usable):
		return Soft
	case p.Ceiling > 0 && size >= p.Ceiling:
		return Soft
	default:
		return Below
	}
}

// Trigger is the size at which this policy asks for a pass: the compact_at
// ceiling when one is set, otherwise the soft ratio of the window a turn can
// fill. Zero means there is no threshold to cross.
func (p Policy) Trigger() int64 {
	if p.Ceiling > 0 {
		return p.Ceiling
	}
	if usable := p.Usable(); usable > 0 {
		return int64(p.Ratios.Soft * float64(usable))
	}
	return 0
}

// reaches compares against a rounded threshold rather than the raw product, so
// a ratio like 0.65 of 200000 lands exactly on 130000 instead of just above it.
func reaches(size int64, ratio float64, window int64) bool {
	if ratio <= 0 {
		return false
	}
	return size >= int64(ratio*float64(window)+0.5)
}

// Plan partitions the conversation by index into anchor, ledger, history and
// active spans, in that order. The active boundary is pulled back so it never
// opens on a ToolResult whose call the cut dropped, the pinned head is extended
// over the replies answering the calls it carries so it never splits a pair
// either, and a missing ledger is simply left out — a session has none until its
// first pass builds one. Spans are contiguous and cover the whole conversation;
// only the history spans are eligible for a pass.
func Plan(conv []nacelle.Message, p Policy) []Span {
	n := len(conv)
	anchor := anchorEnd(conv, clamp(p.AnchorMessages, 0, n))
	active := activeStart(conv, n, anchor, p)
	ledger := ledgerIndex(conv, anchor, active)

	spans := make([]Span, 0, 5)
	if anchor > 0 {
		spans = append(spans, Span{Zone: ZoneAnchor, Start: 0, End: anchor})
	}
	start := anchor
	if ledger >= 0 {
		end := LedgerEnd(conv, ledger, active)
		spans = appendHistory(spans, start, ledger)
		spans = append(spans, Span{Zone: ZoneLedger, Start: ledger, End: end})
		start = end
	}
	spans = appendHistory(spans, start, active)
	if active < n {
		spans = append(spans, Span{Zone: ZoneActive, Start: active, End: n})
	}
	return spans
}

// anchorEnd extends the pinned head over the replies answering the tool calls the
// head itself carries, the same way a ledger claims its own. Without it a
// conversation whose head is an assistant ToolCall leaves the matching ToolResult
// as the first history block — an orphan block a later prune or fold would drop
// while its call stayed pinned in the anchor, which the provider rejects. A head
// that asked for nothing is returned unchanged.
func anchorEnd(conv []nacelle.Message, anchor int) int {
	if anchor <= 0 || anchor >= len(conv) {
		return anchor
	}
	calls := toolCallIDs(conv[anchor-1])
	end := anchor
	for end < len(conv) && answersAny(conv[end], calls) {
		end++
	}
	return end
}

func ledgerIndex(conv []nacelle.Message, anchor, active int) int {
	for i := anchor; i < active; i++ {
		if IsLedger(conv[i]) {
			return i
		}
	}
	return -1
}
