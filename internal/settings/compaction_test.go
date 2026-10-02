package settings

import (
	"strings"
	"testing"
)

// The ladder, the tail and the judge's own defaults are the shipped policy: the
// judge is off because enabling it is what sends history off the machine. The
// window and the reserve are the one pair deliberately left unset — the first is
// the backend's own window and the second a fifth of whatever that turns out to
// be, neither of which this layer can know.
func TestCompactionDefaults(t *testing.T) {
	c := Defaults("").Compaction
	soft, smart := c.Ratios()
	if soft != DefaultSoftRatio || smart != DefaultSmartRatio {
		t.Errorf("ratios = %v/%v, want %v/%v", soft, smart, DefaultSoftRatio, DefaultSmartRatio)
	}
	if c.KeepTurns == nil || *c.KeepTurns != 1 {
		t.Errorf("keep_turns = %v, want the floor of 1", c.KeepTurns)
	}
	if c.KeepTokens == nil || *c.KeepTokens != DefaultKeepTokens {
		t.Errorf("keep_tokens = %v, want the shipped %d", c.KeepTokens, DefaultKeepTokens)
	}
	if c.AnchorMessages == nil || *c.AnchorMessages != 1 {
		t.Errorf("anchor_messages = %v, want 1", c.AnchorMessages)
	}
	if c.WindowTokens != nil || c.ReserveTokens != nil {
		t.Errorf("window/reserve = %v/%v, want both unset", c.WindowTokens, c.ReserveTokens)
	}
	if DerefBool(c.Judge.Enabled) {
		t.Error("judge enabled = true, want the judge off by default")
	}
	if c.Judge.Model != "jev-latest" || c.Judge.BaseURL != "https://api.typesafe.ai" {
		t.Errorf("judge endpoint = %q %q, want the TypeSafe defaults", c.Judge.Model, c.Judge.BaseURL)
	}
	if c.Judge.PruneThreshold == nil || *c.Judge.PruneThreshold != DefaultPruneThreshold {
		t.Errorf("prune_threshold = %v, want the shipped %v", c.Judge.PruneThreshold, DefaultPruneThreshold)
	}
	if c.Judge.MaxBlocks == nil || *c.Judge.MaxBlocks != 64 {
		t.Errorf("max_blocks_per_call = %v, want 64", c.Judge.MaxBlocks)
	}
}

// A file that mentions one ratio leaves the rest of the ladder standing, the
// way every other pointer setting in this package behaves — and its silence
// about compact_at must not resurrect a non-zero default.
func TestCompactionFileLeavesUnmentionedRatiosAlone(t *testing.T) {
	written(t, "limits:\n  compaction:\n    soft_ratio: 0.5\n    judge:\n      enabled: true\n")
	c, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	soft, smart := c.Compaction.Ratios()
	if soft != 0.5 || smart != DefaultSmartRatio {
		t.Errorf("ratios = %v/%v, want the file's 0.5 then the defaults", soft, smart)
	}
	if !DerefBool(c.Compaction.Judge.Enabled) {
		t.Error("judge enabled = false, want the file's true")
	}
	if c.CompactAt != nil {
		t.Errorf("compact_at = %d, want the file's silence to leave it unset", *c.CompactAt)
	}
}

// The ladder is a chain of comparisons over one number, so a ratio outside (0,1]
// or a rung below the one under it is a trigger that either never fires or fires
// out of order — and nothing at run time says so. A mistyped 1.5 looks exactly
// like an enabled compaction that never compacts, and soft_ratio: 0 derives a
// zero ceiling, which every gate reads as "compaction off". Both are refused at
// load rather than discovered later.
//
// NaN is the case the range tests used to miss: it compares false against every
// bound, so `x <= 0 || x > 1` let it through and the ladder's own
// `size >= ratio*window` was then true at every size, pinning the session at the
// top tier while the file still read as an ordinary ladder. YAML's `.nan` is the
// spelling that reaches it.
func TestCompactionRejectsAnUnusableLadder(t *testing.T) {
	tests := map[string]string{
		"a ratio above one":                        "limits:\n  compaction:\n    smart_ratio: 1.5\n",
		"a zero soft ratio":                        "limits:\n  compaction:\n    soft_ratio: 0\n",
		"a negative ratio":                         "limits:\n  compaction:\n    smart_ratio: -0.2\n",
		"a NaN soft ratio":                         "limits:\n  compaction:\n    soft_ratio: .nan\n",
		"a NaN smart ratio":                        "limits:\n  compaction:\n    smart_ratio: .nan\n",
		"rungs out of order":                       "limits:\n  compaction:\n    soft_ratio: 0.9\n",
		"a zero prune threshold":                   "limits:\n  compaction:\n    judge:\n      prune_threshold: 0\n",
		"a prune threshold above one":              "limits:\n  compaction:\n    judge:\n      prune_threshold: 1.2\n",
		"a NaN prune threshold":                    "limits:\n  compaction:\n    judge:\n      prune_threshold: .nan\n",
		"a zero tail budget":                       "limits:\n  compaction:\n    keep_tokens: 0\n",
		"a negative tail budget":                   "limits:\n  compaction:\n    keep_tokens: -1\n",
		"a zero message floor":                     "limits:\n  compaction:\n    keep_turns: 0\n",
		"a reserve that fills the declared window": "limits:\n  compaction:\n    window_tokens: 100000\n    reserve_tokens: 100000\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			written(t, body)
			if _, err := settings(Config{}); err == nil {
				t.Error("settings accepted the ladder, want a load error naming the key")
			}
		})
	}
}

// A job file's own compaction block merges over the resolved config after
// Settings has run, so it is validated where the job is decoded — otherwise a
// job file would be the one surface where an unusable ladder stays silent.
func TestJobFileCompactionIsValidated(t *testing.T) {
	if _, err := decodeJob([]byte("prompt: hi\nlimits:\n  compaction:\n    soft_ratio: 0\n"), "job.yml"); err == nil {
		t.Error("decodeJob accepted a job ladder that cannot work")
	}
	if _, err := decodeJob([]byte("prompt: hi\nlimits:\n  compaction:\n    soft_ratio: 0.5\n"), "job.yml"); err != nil {
		t.Errorf("decodeJob rejected a usable job ladder: %v", err)
	}
}

// A negative compact_at is neither the spelling for off nor a ceiling a session
// could act on: every gate tests `> 0`, so a negative reads as "compaction off"
// while the file still shows a configured ceiling, and it takes the overflow
// recovery with it. 0 is the one way to say off, so a negative is refused rather
// than reinterpreted.
func TestCompactionRejectsANegativeCompactAt(t *testing.T) {
	written(t, "limits:\n  compact_at: -1\n")
	_, err := settings(Config{})
	if err == nil {
		t.Fatal("settings accepted a negative compact_at, want a load error")
	}
	if !strings.Contains(err.Error(), "limits.compact_at") {
		t.Errorf("error = %q, want it to name limits.compact_at", err)
	}
	for _, body := range []string{"limits:\n  compact_at: 0\n", "limits:\n  compact_at: 120000\n"} {
		written(t, body)
		if _, err := settings(Config{}); err != nil {
			t.Errorf("settings rejected %q: %v", body, err)
		}
	}
}

// The tail's bounds and the two window figures reach a resolved session the way
// every other pointer does: the environment can carry them, and each falls back
// to the shipped default on its own rather than taking the rest with it.
func TestCompactionCarriesTheTailAndTheWindowOverride(t *testing.T) {
	written(t, "")
	t.Setenv(EnvPrefix+"COMPACTION_KEEP_TOKENS", "20000")
	t.Setenv(EnvPrefix+"COMPACTION_RESERVE_TOKENS", "32000")
	t.Setenv(EnvPrefix+"COMPACTION_WINDOW_TOKENS", "300000")

	c, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if got := DerefInt64(c.Compaction.KeepTokens); got != 20_000 {
		t.Errorf("keep_tokens = %d, want the environment's 20000", got)
	}
	if got := DerefInt64(c.Compaction.ReserveTokens); got != 32_000 {
		t.Errorf("reserve_tokens = %d, want the environment's 32000", got)
	}
	if got := DerefInt64(c.Compaction.WindowTokens); got != 300_000 {
		t.Errorf("window_tokens = %d, want the environment's 300000", got)
	}
	if got := DerefInt(c.Compaction.KeepTurns); got != DefaultKeepTurns {
		t.Errorf("keep_turns = %d, want the default when the environment is silent", got)
	}
}

// A ladder somebody can actually run still loads, so the gate above is a
// judgement on the values and not a refusal to have any: a rung at exactly 1 is
// the inclusive end of the range, not a mistake.
func TestCompactionAcceptsAUsableLadder(t *testing.T) {
	written(t, "limits:\n  compaction:\n    soft_ratio: 0.5\n    smart_ratio: 1\n    judge:\n      prune_threshold: 0.9\n")

	if _, err := settings(Config{}); err != nil {
		t.Errorf("settings rejected a usable ladder: %v", err)
	}
}

// The judge's key can live in the file like any other setting: an environment
// that mentions neither vendor variable must not clear it, and the vendor's own
// variable must still win over it when it is set.
func TestJudgeKeyComesFromTheFileUnlessTheEnvironmentOverrides(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv(EnvPrefix+"COMPACTION_JUDGE_API_KEY", "")
	written(t, "limits:\n  compaction:\n    judge:\n      enabled: true\n      api_key: from-file\n")

	c, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.APIKey != "from-file" {
		t.Errorf("judge key = %q, want the file's own", c.Compaction.Judge.APIKey)
	}

	t.Setenv("TYPESAFE_API_KEY", "from-env")
	c, err = settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if c.Compaction.Judge.APIKey != "from-env" {
		t.Errorf("judge key = %q, want the environment to win", c.Compaction.Judge.APIKey)
	}
}

// TYPESAFE_API_KEY is the vendor's own name and wins over the namespaced
// setting, so a key already exported for TypeSafe needs no second copy.
func TestJudgeKeyPrefersTheTypesafeVariable(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "from-typesafe")
	t.Setenv(EnvPrefix+"COMPACTION_JUDGE_API_KEY", "from-bulle")
	if key := FromEnv().Compaction.Judge.APIKey; key != "from-typesafe" {
		t.Errorf("judge key = %q, want TYPESAFE_API_KEY to win", key)
	}
	t.Setenv("TYPESAFE_API_KEY", "")
	if key := FromEnv().Compaction.Judge.APIKey; key != "from-bulle" {
		t.Errorf("judge key = %q, want the namespaced variable when TypeSafe's is empty", key)
	}
}

// The environment reaches the same floats through strconv.ParseFloat, which
// accepts "nan" as readily as YAML accepts ".nan". The guards are written in the
// positive form precisely so both spellings are refused, so the environment path
// is pinned rather than assumed to inherit the file's check.
func TestCompactionRejectsANaNFromTheEnvironment(t *testing.T) {
	for _, key := range []string{"SOFT_RATIO", "SMART_RATIO", "PRUNE_THRESHOLD"} {
		t.Run(key, func(t *testing.T) {
			written(t, "")
			t.Setenv(EnvPrefix+"COMPACTION_"+key, "nan")
			if _, err := settings(Config{}); err == nil {
				t.Errorf("settings accepted a NaN %s from the environment", key)
			}
		})
	}
}
