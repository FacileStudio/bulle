package agent

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/settings"
)

// fixedWindow is the package's answeringStub with a context window, so the
// resolver's ratio branch can be exercised without a real backend.
type fixedWindow struct {
	answeringStub
	window int64
}

func (w fixedWindow) Capabilities() nacelle.Capabilities {
	return nacelle.Capabilities{ContextWindow: w.window}
}

// The ceiling has three sources and this pins the order: a value someone set
// wins, and 0 in it still turns compaction off; an unset value derives from the
// soft ratio and the window a turn can fill; a windowless backend falls back to
// the constant.
//
// The derived rung is the soft ratio of the *usable* window, not the raw one: a
// 200000 window holds back a 40000 reserve for the answer, so the ladder is read
// against 160000 and soft lands at 104000 rather than 130000.
func TestResolveBudgetCeilingPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		compactAt *int64
		window    int64
		ceiling   int64
	}{
		{"unset derives from the usable window", nil, 200_000, 104_000},
		{"an explicit value wins", new(int64(50_000)), 200_000, 50_000},
		{"an explicit 0 still disables", new(int64(0)), 200_000, 0},
		{"a windowless backend falls back", nil, 0, settings.DefaultCompactAt},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBudget(tc.compactAt, settings.Compaction{}, &fixedWindow{window: tc.window})
			if got.Ceiling != tc.ceiling {
				t.Errorf("ceiling = %d, want %d", got.Ceiling, tc.ceiling)
			}
			if got.Window != tc.window {
				t.Errorf("window = %d, want %d", got.Window, tc.window)
			}
		})
	}
}

// The ladder is carried whether or not compact_at overrode the ceiling, so a
// session that pins the trigger still tiers its passes.
func TestResolveBudgetCarriesTheRatios(t *testing.T) {
	soft, smart := 0.5, 0.7
	configured := settings.Compaction{SoftRatio: &soft, SmartRatio: &smart}
	got := ResolveBudget(new(int64(80_000)), configured, &fixedWindow{window: 100_000})
	if got.TierRatio.Soft != 0.5 || got.TierRatio.Smart != 0.7 {
		t.Errorf("ratios = %+v, want the configured ladder", got.TierRatio)
	}
	if got.Ceiling != 80_000 {
		t.Errorf("ceiling = %d, want the pinned 80000", got.Ceiling)
	}
}

// The reserve is the runway held back from the window for the turn's own
// answer, and the ladder is read against what is left of it. It defaults to a
// fifth of the window inside the shipped bounds; a value somebody set wins,
// except that one large enough to swallow the window is cut to a quarter of it,
// because a reserve that leaves nothing to reserve from is compaction switched
// off by arithmetic rather than a preference.
func TestResolveBudgetHoldsBackAReserve(t *testing.T) {
	tests := []struct {
		name       string
		window     int64
		configured *int64
		reserve    int64
		usable     int64
	}{
		{"a fifth of an ordinary window", 200_000, nil, 40_000, 160_000},
		{"floored on a small window", 32_000, nil, 8_192, 23_808},
		{"capped on a very large one", 1_000_000, nil, 65_536, 934_464},
		{"an explicit value wins", 200_000, new(int64(12_000)), 12_000, 188_000},
		{"one that would swallow the window is cut", 200_000, new(int64(500_000)), 50_000, 150_000},
		{"a windowless backend has nothing to take one from", 0, nil, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBudget(nil, settings.Compaction{ReserveTokens: tc.configured}, &fixedWindow{window: tc.window})
			if got.Reserve != tc.reserve {
				t.Errorf("reserve = %d, want %d", got.Reserve, tc.reserve)
			}
			if got.Usable != tc.usable {
				t.Errorf("usable = %d, want %d", got.Usable, tc.usable)
			}
		})
	}
}

// window_tokens is the figure the ladder is measured against when a backend
// under-reports one or reports nothing at all — the OpenAI-compatible runner
// reports zero, which otherwise leaves every session on one global 75000.
func TestResolveBudgetWindowOverride(t *testing.T) {
	set := ResolveBudget(nil, settings.Compaction{WindowTokens: new(int64(300_000))}, &fixedWindow{window: 50_000})
	if set.Window != 300_000 {
		t.Errorf("window = %d, want the configured 300000 to beat the reported 50000", set.Window)
	}
	if set.Reserve != 60_000 || set.Usable != 240_000 {
		t.Errorf("reserve/usable = %d/%d, want 60000/240000", set.Reserve, set.Usable)
	}
	if set.Ceiling != 156_000 {
		t.Errorf("ceiling = %d, want soft_ratio of the usable window", set.Ceiling)
	}

	blind := ResolveBudget(nil, settings.Compaction{WindowTokens: new(int64(300_000))}, &fixedWindow{window: 0})
	if blind.Window != 300_000 || blind.Ceiling != 156_000 {
		t.Errorf("window/ceiling = %d/%d, want the declared window to stand in for the missing one", blind.Window, blind.Ceiling)
	}
}

// A derived ceiling that comes out zero would be read by every gate as
// "compaction off", so a degenerate ratio falls back to the windowless default
// rather than quietly disabling the ladder. Settings refuses such a ratio at
// load, which leaves this as the floor under a caller that built a Compaction
// itself — the shape a test or a programmatic embed reaches.
func TestResolveBudgetNeverDerivesAZeroCeiling(t *testing.T) {
	zero, negative := 0.0, -0.5

	for _, soft := range []*float64{&zero, &negative} {
		got := ResolveBudget(nil, settings.Compaction{SoftRatio: soft}, &fixedWindow{window: 200_000})
		if got.Ceiling != settings.DefaultCompactAt {
			t.Errorf("ceiling = %d for soft_ratio %v, want %d — a zero ceiling reads as disabled",
				got.Ceiling, *soft, settings.DefaultCompactAt)
		}
	}
}

// A window and a reserve written together reach the budget as written: the reserve
// is held back and what remains is what the ladder measures. Settings refuses the
// pair whose remainder could not tier, so this pins the arithmetic on one that
// loads, through the loader rather than a struct built in the test.
func TestResolveBudgetCarriesADeclaredWindowAndReserve(t *testing.T) {
	writeHome(t, "limits:\n  compaction:\n    window_tokens: 200000\n    reserve_tokens: 80000\n")

	cfg := resolve(t)
	budget := ResolveBudget(cfg.CompactAt, cfg.Compaction, &fixedWindow{window: 50_000})
	if budget.Window != 200_000 || budget.Reserve != 80_000 || budget.Usable != 120_000 {
		t.Errorf("window/reserve/usable = %d/%d/%d, want 200000/80000/120000",
			budget.Window, budget.Reserve, budget.Usable)
	}
	if budget.Ceiling != 78_000 {
		t.Errorf("ceiling = %d, want soft_ratio of the 120000 a turn can fill", budget.Ceiling)
	}
}

// A reserve is cut whenever it would leave too little for the ladder to measure
// against, not only when it swallows the window whole. One token under the window
// leaves a single usable token, and every tier then fires at every size while the
// config reads as a healthy reserve. The load-time check cannot see a backend that
// reports its own window, so the same rule has to hold here.
func TestResolveReserveCutsAReserveThatStarvesTheWindow(t *testing.T) {
	tests := []struct {
		name       string
		window     int64
		configured int64
		want       int64
	}{
		{"a reserve one under the window is cut to a quarter", 200_000, 199_999, 50_000},
		{"half the window is the most a reserve may take", 200_000, 100_000, 100_000},
		{"a modest reserve is kept as written", 200_000, 80_000, 80_000},
		{"a windowless backend reserves nothing", 0, 80_000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveReserve(tc.window, tc.configured); got != tc.want {
				t.Errorf("resolveReserve(%d, %d) = %d, want %d", tc.window, tc.configured, got, tc.want)
			}
		})
	}
}

// The half-cap bounds the token budget and not the message floor. activeStart lays
// KeepTurns down before the budget widens anything, and the active window is never
// rewritten, summarized or pruned by any tier — so a floor somebody set keeps the
// tail over half of what a pass may fill however heavy the turns under it are.
// That is the one bound no pass can move, and it is pinned through the real loader
// and the policy it resolves to rather than a hand-built policy.
func TestResolvedTailHonoursTheMessageFloorOverTheHalfCap(t *testing.T) {
	tests := []struct {
		name string
		body string
		kept int
	}{
		{"the shipped floor keeps the newest turn", "limits:\n  compaction:\n    window_tokens: 200000\n", 1},
		{"a floor of three outranks the half-cap", "limits:\n  compaction:\n    window_tokens: 200000\n    keep_turns: 3\n", 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writeHome(t, tc.body)
			cfg := resolve(t)
			budget := ResolveBudget(cfg.CompactAt, cfg.Compaction, &fixedWindow{window: 200_000})
			policy := CompactionConfig(budget, cfg.Compaction).Policy
			if policy.KeepTurns != tc.kept {
				t.Errorf("resolved keep_turns = %d, want the loaded %d", policy.KeepTurns, tc.kept)
			}

			conv := []nacelle.Message{nacelle.UserText("the task")}
			for range 4 {
				conv = append(conv, nacelle.AssistantText(strings.Repeat("x", 400_000)))
			}
			active := compaction.Section(conv, compaction.Plan(conv, policy), compaction.ZoneActive)
			if len(active) != tc.kept {
				t.Errorf("the active window keeps %d messages, want the floor of %d to outrank the cap",
					len(active), tc.kept)
			}
		})
	}
}
