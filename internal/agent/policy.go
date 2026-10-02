package agent

import (
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

// Judge builds the opt-in System One classifier from the compaction settings, or
// nil while the judge is off — the default, because enabling it sends
// conversation history to a third party. The key prefers TYPESAFE_API_KEY, which
// the settings layer has already resolved into the config.
func Judge(c settings.Compaction) compaction.Judge {
	judge := c.Judge
	if !settings.DerefBool(judge.Enabled) {
		return nil
	}
	return compaction.NewJevJudge(compaction.JudgeConfig{
		Enabled:        true,
		Model:          judge.Model,
		BaseURL:        judge.BaseURL,
		APIKey:         judge.APIKey,
		PruneThreshold: settings.DerefFloat(judge.PruneThreshold),
		MaxBlocks:      settings.DerefInt(judge.MaxBlocks),
	})
}
