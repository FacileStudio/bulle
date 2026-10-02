package settings

import (
	"fmt"

	"github.com/FacileStudio/bulle/internal/compaction"
)

// validateTail rejects a tail bound or a window figure that cannot work. It runs
// on the resolved settings, so only a value a layer actually wrote down is
// judged — a job file that mentions one ratio and nothing else keeps the shipped
// tail rather than failing on the fields it never named.
//
// Each rejected value is a number a zero would quietly disable rather than a
// preference. `keep_tokens: 0` leaves the tail sized by keep_turns alone,
// `window_tokens: 0` reads as "ask the backend" — which is what leaving it out
// already means — and a reserve that leaves under half of a declared window
// usable shrinks the ladder to a single rung: usable collapses toward one token,
// every tier trips at once, and the session sits at the top tier while the config still
// reads as enabled. That is the same failure `soft_ratio: 0` used to have, and
// failing at load is what keeps it from being found out later.
//
// The reserve and window are only compared with each other when both were
// written down here. The window usually comes from the backend, which this layer
// cannot see, so ResolveBudget cuts a reserve that would swallow it instead.
//
// The message floor is judged the same way, against the same cap the token budget
// is held to. A `keep_turns` whose own turns already weigh more than half of what
// a pass may fill keeps the tail over that cap whatever the budget is: the floor
// is laid down before the budget widens anything, and the active window is never
// rewritten, summarized or pruned by any tier, so the pass the tail triggers
// could never land under its own trigger. What the floor costs is a count of
// turns nobody has taken yet, so it is judged on the package's own estimate of
// one — see refuseFloorOverCap.
func validateTail(c Compaction, compactAt *int64) error {
	if c.KeepTurns != nil && *c.KeepTurns < 1 {
		return &ParseError{Path: "limits.compaction.keep_turns", Err: fmt.Errorf(
			"want a count of at least 1, got %d", *c.KeepTurns)}
	}
	if c.AnchorMessages != nil && *c.AnchorMessages < 1 {
		return &ParseError{Path: "limits.compaction.anchor_messages", Err: fmt.Errorf(
			"want a count of at least 1, got %d", *c.AnchorMessages)}
	}
	for _, bound := range []struct {
		key   string
		value *int64
	}{{"keep_tokens", c.KeepTokens}, {"reserve_tokens", c.ReserveTokens}, {"window_tokens", c.WindowTokens}} {
		if bound.value != nil && *bound.value < 1 {
			return &ParseError{Path: "limits.compaction." + bound.key, Err: fmt.Errorf(
				"want a positive token count, got %d — leave it out and bulle derives it", *bound.value)}
		}
	}
	if c.ReserveTokens != nil && c.WindowTokens != nil && *c.ReserveTokens > *c.WindowTokens/2 {
		return &ParseError{Path: "limits.compaction.reserve_tokens", Err: fmt.Errorf(
			"want a reserve that leaves at least half of window_tokens usable, got %d of %d", *c.ReserveTokens, *c.WindowTokens)}
	}
	return refuseFloorOverCap(c, compactAt)
}

// refuseFloorOverCap rejects a message floor whose own estimated weight is over
// the cap the token budget is held to: half of the smaller of the usable window
// and the ceiling, which is the figure keepBudget clamps the budget to. A floor
// that heavy is what the cap exists to prevent, and no pass can move it — the
// active window is never rewritten, summarized or pruned — so a config promising
// one is refused rather than left to sit in the thrash guard.
//
// Only what a layer wrote down here bounds the cap, the same way the reserve is
// judged. Either figure bounds the cap on its own: the usable window can never
// exceed the window_tokens override, and the ceiling can never exceed the
// compact_at this layer is handed, so whichever of the two a session wrote down
// is what the floor is measured against. With neither written the window is the
// backend's own and this layer judges nothing, because a check that refused a
// config over a window it cannot see would be a worse bug than the floor.
//
// One message is estimated at compaction.EstMsgTokens: a floor is a count of
// turns the session has not taken, so there is nothing to measure, and the
// package's bound for one unit of history is what stands in for one. It is the
// right proxy for an unwritten turn because a turn is what a block is — a tool
// call with the reply answering it, or a standalone turn — and the tool result in
// it is exactly the weight that bound exists for.
func refuseFloorOverCap(c Compaction, compactAt *int64) error {
	cap, known := halfCap(c, compactAt)
	if !known || c.KeepTurns == nil {
		return nil
	}
	floor := int64(*c.KeepTurns) * compaction.EstMsgTokens
	if floor <= cap {
		return nil
	}
	return &ParseError{Path: "limits.compaction.keep_turns", Err: fmt.Errorf(
		"want a message floor whose estimated weight is under half of what a pass may fill, got about %d tokens of %d", floor, cap)}
}

// halfCap is the most the cap on the tail can be from what this layer was handed,
// and whether it can be put to a number at all. The cap is half of the smaller of
// the usable window and the ceiling; the window_tokens override bounds the first
// from above, the reserve below it only lowers it, and compact_at bounds the
// second. Zero means neither was written down, which is the backend's own window
// and a ceiling derived from it: nothing here bounds the cap, so nothing here is
// refused.
func halfCap(c Compaction, compactAt *int64) (int64, bool) {
	limit := int64(0)
	if c.WindowTokens != nil {
		limit = *c.WindowTokens
		if c.ReserveTokens != nil {
			limit -= *c.ReserveTokens
		}
	}
	if compactAt != nil && *compactAt > 0 && (limit <= 0 || *compactAt < limit) {
		limit = *compactAt
	}
	if limit <= 0 {
		return 0, false
	}
	return limit / 2, true
}
