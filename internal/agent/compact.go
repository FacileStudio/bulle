package agent

import (
	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

// Budget is the resolved compaction ceiling for one session: the token count the
// soft tier trips at, the ratio ladder the smart tier sits on, the
// backend's total window (0 when the backend does not report one), and the
// runway held back from it.
type Budget struct {
	Ceiling   int64
	TierRatio struct {
		Soft, Smart float64
	}
	Window int64
	// Reserve is the part of the window kept free for the turn's own answer, and
	// Usable is what is left of it — the figure the ladder is measured against.
	// Reserve is 0 when there is no window to take it from.
	Reserve int64
	Usable  int64
}

// The reserve's shape when a session does not name one: a fifth of the window,
// floored so a small window still keeps room for a turn's answer, and capped so
// a large one does not hold back more than any single turn could use.
//
// A fifth is a hypothesis rather than a measurement, and the honest way to read
// it is as a starting point: what a turn needs is its generation ceiling,
// which no backend reports through Capabilities, so the number is derived from
// the window and a session that knows better should set reserve_tokens. The
// direction it errs in is the safe one — a reserve too large compacts a little
// earlier than it had to.
const (
	minReserve     = 8 * 1024
	maxReserve     = 64 * 1024
	reserveDivisor = 5
)

// ResolveBudget turns the compact_at setting and the compaction ratios into the
// ceiling a session actually runs on. A value someone set — file, environment
// or flag — wins outright, and 0 in it still turns compaction off. An unset
// value derives the ceiling from soft_ratio × the window a turn can fill, and
// falls back to settings.DefaultCompactAt when the backend reports no window at
// all. The ratio ladder is carried whether or not compact_at overrode the
// ceiling, so a session that pins compact_at still tiers its passes.
//
// The reserve is subtracted before the ladder is read rather than from each rung
// in turn: a ratio names a fraction of the window a session can actually fill, so
// smart_ratio 0.80 leaves a fifth of the *usable* window plus the whole reserve for
// the response, instead of a fifth of the raw one.
//
// A derived ceiling that comes out zero or less falls back to
// settings.DefaultCompactAt rather than staying at zero, because a ceiling of
// zero is how every gate spells "compaction off": letting a degenerate ratio
// produce one would turn the ladder off while reporting it as enabled. Settings
// rejects such a ratio at load, so this is the floor under a caller that built a
// Compaction itself.
func ResolveBudget(compactAt *int64, c settings.Compaction, backend nacelle.Backend) Budget {
	soft, smart := c.Ratios()
	window := resolveWindow(backend, c.WindowTokens)
	budget := Budget{Window: window}
	budget.TierRatio.Soft, budget.TierRatio.Smart = soft, smart
	budget.Reserve = resolveReserve(window, settings.DerefInt64(c.ReserveTokens))
	budget.Usable = max(window-budget.Reserve, 0)
	switch {
	case compactAt != nil:
		budget.Ceiling = *compactAt
	case budget.Usable > 0:
		budget.Ceiling = int64(soft * float64(budget.Usable))
		if budget.Ceiling <= 0 {
			budget.Ceiling = settings.DefaultCompactAt
		}
	default:
		budget.Ceiling = settings.DefaultCompactAt
	}
	return budget
}

// resolveWindow is the context window a session measures against: an explicit
// window_tokens when one is set, otherwise whatever the backend reports.
//
// The override exists because a backend's own number is not always the route's.
// Gateways under-report, OpenAI-compatible runners report nothing at all — which
// leaves the ladder with nothing to measure and every session on one global
// 75000 — and a session deliberately pinned to less than the model advertises is
// a legitimate choice, since a smaller working set costs less and reasons better.
func resolveWindow(backend nacelle.Backend, override *int64) int64 {
	if explicit := settings.DerefInt64(override); explicit > 0 {
		return explicit
	}
	return backend.Capabilities().ContextWindow
}

// resolveReserve is the runway a turn is given for its own answer: the value a
// session set, or a fifth of the window within the shipped bounds. A reserve
// that would swallow the window is cut to a quarter of it — a reserve that
// leaves nothing to reserve from is not a reserve, it is compaction switched off
// by arithmetic — and a windowless backend has nothing to take one from.
func resolveReserve(window, configured int64) int64 {
	if window <= 0 {
		return 0
	}
	reserve := configured
	if reserve <= 0 {
		reserve = min(max(window/reserveDivisor, minReserve), maxReserve)
	}
	if reserve > window/2 {
		reserve = window / 4
	}
	return reserve
}
