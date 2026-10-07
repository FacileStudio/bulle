package agent

import (
	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/tui"
)

// Policy folds the resolved budget and the tail's own bounds into the one
// compaction policy an interactive session reads. It keeps the ratios even when
// compact_at overrode the ceiling, so a session that pins its trigger still
// tiers its passes against the window, and carries the reserve so the tier the
// trigger reads is measured on the same figure the budget resolved.
func Policy(b Budget, keepTurns int, keepTokens int64, anchorMessages int) compaction.Policy {
	return compaction.Policy{
		Ratios:         compaction.Ratios{Soft: b.TierRatio.Soft, Smart: b.TierRatio.Smart},
		Window:         b.Window,
		Reserve:        b.Reserve,
		Ceiling:        b.Ceiling,
		KeepTurns:      keepTurns,
		KeepTokens:     keepTokens,
		AnchorMessages: anchorMessages,
	}
}

// CompactionConfig folds the resolved budget, the tail's bounds and the opt-in
// judge into the one struct a session reads, so runflags stays one line per
// field.
func CompactionConfig(b Budget, c settings.Compaction) tui.CompactionConfig {
	turns, tokens, anchor := tailBounds(c)
	return tui.CompactionConfig{
		Policy: Policy(b, turns, tokens, anchor),
		Judge:  Judge(c),
	}
}

// tailBounds are the message floor, the token budget and the pinned head, with
// the shipped defaults filling in whatever a config left out. A Compaction built
// in code rather than resolved from the settings chain carries zeroes, and a
// zeroed tail pins nothing: no floor under the newest turn and no budget over it.
func tailBounds(c settings.Compaction) (turns int, tokens int64, anchor int) {
	turns, tokens, anchor = settings.DerefInt(c.KeepTurns), settings.DerefInt64(c.KeepTokens), settings.DerefInt(c.AnchorMessages)
	if turns <= 0 {
		turns = compaction.DefaultKeepTurns
	}
	if tokens <= 0 {
		tokens = compaction.DefaultKeepTokens
	}
	if anchor <= 0 {
		anchor = compaction.DefaultAnchorMessages
	}
	return turns, tokens, anchor
}

// Judge builds the opt-in decision-model classifier from the compaction
// settings, or nil while the judge is off — the default, because enabling it
// sends conversation history to a third party. The provider picks the endpoint
// and the defaults for model and host through compaction.SpecFor; an explicit
// model or base URL wins over them, which is the escape hatch for a proxy or a
// pinned build. The key prefers the vendor's own environment variable and the
// settings layer has already resolved it into the config.
func Judge(c settings.Compaction) compaction.Judge {
	judge := c.Judge
	if !settings.DerefBool(judge.Enabled) {
		return nil
	}
	spec := compaction.SpecFor(judge.Provider)
	return compaction.NewJevJudge(compaction.JudgeConfig{
		Enabled:        true,
		Endpoint:       spec.Endpoint,
		Model:          firstSetting(judge.Model, spec.Model),
		BaseURL:        firstSetting(judge.BaseURL, spec.BaseURL),
		APIKey:         judge.APIKey,
		PruneThreshold: settings.DerefFloat(judge.PruneThreshold),
		MaxBlocks:      settings.DerefInt(judge.MaxBlocks),
	})
}

// firstSetting is an explicit value over a derived default: a model or host
// anybody wrote down wins over what the provider suggests.
func firstSetting(explicit, derived string) string {
	if explicit != "" {
		return explicit
	}
	return derived
}

// ChatCompactor builds a compaction Engine for an agent and settings.
func ChatCompactor(config settings.Config, a *nacelle.Agent) *compaction.Engine {
	turns, tokens, anchor := tailBounds(config.Compaction)
	soft, smart := config.Compaction.Ratios()
	ceiling := settings.DefaultCompactAt
	if config.CompactAt != nil {
		ceiling = *config.CompactAt
	}
	var window int64
	if config.Compaction.WindowTokens != nil {
		window = *config.Compaction.WindowTokens
	}
	policy := compaction.Policy{
		Ratios:         compaction.Ratios{Soft: soft, Smart: smart},
		Window:         window,
		Ceiling:        ceiling,
		KeepTurns:      turns,
		KeepTokens:     tokens,
		AnchorMessages: anchor,
	}
	builder := func() (*nacelle.Agent, error) { return a, nil }
	return compaction.NewEngine(policy, Judge(config.Compaction), builder)
}
