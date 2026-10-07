package settings

import (
	"fmt"
	"os"
)

// Compaction is the ratio-based context-management surface: the tier ladder
// that decides when a session tombstones, prunes or folds history, the two
// figures that say how much window the ladder is really measured against, and
// the opt-in judge that classifies history blocks before any of that happens.
// Every scalar is a pointer, so a layer that mentions one ratio leaves the rest
// of the policy alone instead of resetting it to zero.
type Compaction struct {
	SoftRatio  *float64 `yaml:"soft_ratio"`
	SmartRatio *float64 `yaml:"smart_ratio"`
	// WindowTokens overrides the backend's reported context window, and
	// ReserveTokens is the part of that window held back for the turn's own
	// answer. The ratios are read against the window minus the reserve, so the
	// reserve is what keeps the top rung from leaving the model nothing to answer
	// with. Both are optional: an unset window is whatever the backend reports, and
	// an unset reserve is a fifth of that window within the shipped bounds.
	WindowTokens  *int64 `yaml:"window_tokens"`
	ReserveTokens *int64 `yaml:"reserve_tokens"`
	// KeepTurns is a floor in messages the verbatim tail never drops below, and
	// KeepTokens is the budget that sizes it beyond that floor. The floor is the
	// smaller guarantee on purpose: a message floor that is generous is the one
	// bound neither tier can move, because the newest turns are where a session's
	// largest tool results land and the active window is never touched.
	KeepTurns      *int   `yaml:"keep_turns"`
	KeepTokens     *int64 `yaml:"keep_tokens"`
	AnchorMessages *int   `yaml:"anchor_messages"`
	Judge          Judge  `yaml:"judge"`
}

// Judge is the decision model that ranks history blocks keep / prune /
// ledger before the summarizer writes the ledger. It is off by default:
// enabling it sends conversation history to a third party, so it is an
// explicit opt-in and never a shipped default.
//
// Provider names the decision model: jev is TypeSafe's System One (the
// default), clef is Cloudflare's fine-tune served on Workers AI through
// OpenRouter. Model and BaseURL are derived from it where the judge is built;
// setting them explicitly still wins, which is an escape hatch for pointing
// at a proxy or a versioned build rather than something to advertise. The
// key prefers the vendor's own environment variable over the file, and
// api_key_command is the alternative that leaves a file carrying no
// credential at all (see keycmd.go).
type Judge struct {
	Enabled *bool `yaml:"enabled"`
	// Provider is the decision model: "jev" (TypeSafe direct, the default) or
	// "clef" (Cloudflare through OpenRouter). Empty means the default.
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
	// APIKeyCommand is run to obtain the judge's key when APIKey is empty, the
	// same arrangement the provider's key has. See keycmd.go.
	APIKeyCommand  string   `yaml:"api_key_command"`
	PruneThreshold *float64 `yaml:"prune_threshold"`
	MaxBlocks      *int     `yaml:"max_blocks_per_call"`
}

// Merge overwrites every compaction setting over actually mentions, so a
// profile that sets one ratio does not reset the ladder around it. It is
// exported because a cron job's own limits.compaction has to reach a resolved
// run the same way the rest of this package's merge chain does.
func (c *Compaction) Merge(over Compaction) {
	if over.SoftRatio != nil {
		c.SoftRatio = over.SoftRatio
	}
	if over.SmartRatio != nil {
		c.SmartRatio = over.SmartRatio
	}
	if over.WindowTokens != nil {
		c.WindowTokens = over.WindowTokens
	}
	if over.ReserveTokens != nil {
		c.ReserveTokens = over.ReserveTokens
	}
	if over.KeepTurns != nil {
		c.KeepTurns = over.KeepTurns
	}
	if over.KeepTokens != nil {
		c.KeepTokens = over.KeepTokens
	}
	if over.AnchorMessages != nil {
		c.AnchorMessages = over.AnchorMessages
	}
	c.Judge.merge(over.Judge)
}

// merge overwrites every judge field over actually mentions. An empty string
// never clears one a lower layer supplied, so an environment key survives a
// file that only turns the judge on.
func (j *Judge) merge(over Judge) {
	if over.Enabled != nil {
		j.Enabled = over.Enabled
	}
	if over.Provider != "" {
		j.Provider = over.Provider
	}
	if over.Model != "" {
		j.Model = over.Model
	}
	if over.BaseURL != "" {
		j.BaseURL = over.BaseURL
	}
	if over.APIKey != "" {
		j.APIKey = over.APIKey
	}
	if over.APIKeyCommand != "" {
		j.APIKeyCommand = over.APIKeyCommand
	}
	if over.PruneThreshold != nil {
		j.PruneThreshold = over.PruneThreshold
	}
	if over.MaxBlocks != nil {
		j.MaxBlocks = over.MaxBlocks
	}
}

// Ratios is the tier ladder with any ratio a layer left out filled from the
// shipped defaults, so callers never handle the pointers themselves.
func (c Compaction) Ratios() (soft, smart float64) {
	soft, smart = DefaultSoftRatio, DefaultSmartRatio
	if c.SoftRatio != nil {
		soft = *c.SoftRatio
	}
	if c.SmartRatio != nil {
		smart = *c.SmartRatio
	}
	return soft, smart
}

// defaultCompaction is the tier ladder and judge a session with no opinion of
// its own runs on. The judge is off: it is the one setting that sends history
// off the machine, so a machine nobody opted in on never makes that call. The
// model and base URL stay unset here and are derived from the provider where
// the judge is built, so an explicit provider cannot be silently overridden by
// a default that pre-filled the jev values.
func defaultCompaction() Compaction {
	soft, smart := DefaultSoftRatio, DefaultSmartRatio
	keepTurns, anchorMessages, maxBlocks := DefaultKeepTurns, 1, 64
	keepTokens := int64(DefaultKeepTokens)
	pruneThreshold, judgeEnabled := DefaultPruneThreshold, false
	return Compaction{
		SoftRatio:      &soft,
		SmartRatio:     &smart,
		KeepTurns:      &keepTurns,
		KeepTokens:     &keepTokens,
		AnchorMessages: &anchorMessages,
		Judge: Judge{
			Enabled:        &judgeEnabled,
			Provider:       JudgeProviderJEV,
			PruneThreshold: &pruneThreshold,
			MaxBlocks:      &maxBlocks,
		},
	}
}

// compactionEnv is the compaction layer the environment supplies. Every scalar
// is optional and a value it cannot read is treated as unmentioned, exactly the
// way the rest of the environment layer behaves.
func compactionEnv() Compaction {
	return Compaction{
		SoftRatio:      envFloat(EnvPrefix + "COMPACTION_SOFT_RATIO"),
		SmartRatio:     envFloat(EnvPrefix + "COMPACTION_SMART_RATIO"),
		WindowTokens:   envInt64(EnvPrefix + "COMPACTION_WINDOW_TOKENS"),
		ReserveTokens:  envInt64(EnvPrefix + "COMPACTION_RESERVE_TOKENS"),
		KeepTurns:      envInt(EnvPrefix + "COMPACTION_KEEP_TURNS"),
		KeepTokens:     envInt64(EnvPrefix + "COMPACTION_KEEP_TOKENS"),
		AnchorMessages: envInt(EnvPrefix + "COMPACTION_ANCHOR_MESSAGES"),
		Judge: Judge{
			Enabled:        envBool(EnvPrefix + "COMPACTION_JUDGE"),
			Provider:       envGet("COMPACTION_JUDGE_PROVIDER"),
			Model:          envGet("COMPACTION_JUDGE_MODEL"),
			BaseURL:        envGet("COMPACTION_JUDGE_BASE_URL"),
			APIKey:         judgeKeyEnv(envGet("COMPACTION_JUDGE_PROVIDER")),
			PruneThreshold: envFloat(EnvPrefix + "COMPACTION_PRUNE_THRESHOLD"),
			MaxBlocks:      envInt(EnvPrefix + "COMPACTION_MAX_BLOCKS"),
		},
	}
}

// judgeKeyEnv reads the judge's key, preferring the provider's own vendor
// variable — so a key already exported for that vendor needs no second copy —
// over the namespaced setting when both are set. An unnamed provider is the
// default one: jev.
func judgeKeyEnv(provider string) string {
	if key := vendorJudgeKey(provider); key != "" {
		return key
	}
	return envGet("COMPACTION_JUDGE_API_KEY")
}

// vendorJudgeKey is the vendor's own environment variable for one judge
// provider: clef is reached through OpenRouter and carries its key under that
// vendor's name, everything else is TypeSafe's. It reads the ambient
// environment, not the settings chain, so it can fill a key no layer supplied.
func vendorJudgeKey(provider string) string {
	if provider == JudgeProviderClef {
		return os.Getenv("OPENROUTER_API_KEY")
	}
	return os.Getenv("TYPESAFE_API_KEY")
}

// ValidateCompaction rejects a tier ladder that cannot work. It runs on the
// resolved settings, so a ratio no layer mentioned has already been filled from
// the defaults and only a value somebody actually wrote down is judged.
//
// The ladder is a chain of comparisons over one number, so a ratio outside (0,1]
// or a rung below the one under it is not a preference — it is a trigger that
// never fires or fires out of order, and nothing at run time says so. A typo like
// smart_ratio: 1.5 looks exactly like an enabled compaction that never compacts,
// and soft_ratio: 0 derives a zero ceiling, which every gate reads as "compaction
// off". Failing at load is what keeps either from being found out later.
//
// Every bound is written in the positive form because NaN compares false against
// all of them: `x <= 0 || x > 1` admits a ratio that is not a number, and YAML's
// `.nan` (or an environment "nan") then reaches a ladder whose own
// `size >= ratio*window` is true at every size, pinning the session at the top
// tier. `!(x > 0 && x <= 1)` rejects it with the same message as any other typo.
func ValidateCompaction(c Compaction, compactAt *int64) error {
	if compactAt != nil && *compactAt < 0 {
		return &ParseError{Path: "limits.compact_at", Err: fmt.Errorf(
			"want 0 (compaction off) or a positive token ceiling, got %d", *compactAt)}
	}
	soft, smart := c.Ratios()
	rungs := []struct {
		key   string
		ratio float64
	}{{"soft_ratio", soft}, {"smart_ratio", smart}}
	for _, rung := range rungs {
		if !(rung.ratio > 0 && rung.ratio <= 1) {
			return &ParseError{Path: "limits.compaction." + rung.key, Err: fmt.Errorf(
				"want a ratio in (0,1], got %v — use limits.compact_at: 0 to turn compaction off", rung.ratio)}
		}
	}
	if !(soft <= smart) {
		return &ParseError{Path: "limits.compaction", Err: fmt.Errorf(
			"want soft_ratio <= smart_ratio, got %v/%v", soft, smart)}
	}
	if err := validateTail(c, compactAt); err != nil {
		return err
	}
	return validateJudge(c.Judge)
}
