package compaction

import (
	"slices"

	"github.com/FacileStudio/nacelle"
)

// Snapshot creates an isolated clone of a conversation and its message parts.
func Snapshot(conv []nacelle.Message) []nacelle.Message {
	out := slices.Clone(conv)
	for i := range out {
		out[i].Parts = slices.Clone(out[i].Parts)
	}
	return out
}

// CanEvictionLandUnder reports whether evicting history can land size under trigger.
func CanEvictionLandUnder(size int64, conv []nacelle.Message, plan []Span, trigger int64) bool {
	start, end, ok := HistoryRange(plan)
	if !ok {
		return false
	}
	if trigger <= 0 {
		return true
	}
	freed := EstTokens(Bytes(conv[start:end]))
	return size-freed+MaxLedgerTokens <= trigger
}

// EvictionCanLandUnder reports whether evicting history can land size under the engine trigger.
func (e *Engine) EvictionCanLandUnder(size int64, conv []nacelle.Message, plan []Span) bool {
	return CanEvictionLandUnder(size, conv, plan, e.Policy.Trigger())
}

// MaskFallback runs tombstoning over the history spans if plan covers conv.
func MaskFallback(conv []nacelle.Message, plan []Span) (MicroStats, bool) {
	if !Covers(conv, plan) {
		return MicroStats{}, false
	}
	return Tombstone(conv, plan), true
}
