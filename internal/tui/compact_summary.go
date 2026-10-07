package tui

import (
	"fmt"
	"strings"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/status"
)

type compacted struct {
	evictCut int
	turns    int
	kept     int
	pruned   int
	results  int
	tier     compaction.Tier
	replaced bool
	keptAsIs bool
}

func compactReport(outcome compaction.Outcome, d compacted) string {
	freed := outcome.Before - outcome.After
	kept := 0
	if d.evictCut+d.kept > 0 {
		kept = d.kept * 100 / (d.evictCut + d.kept)
	}

	var work []string
	if d.results > 0 {
		work = append(work, "masked "+countedNoun(d.results, "result"))
	}
	if d.turns > 0 {
		work = append(work, "summarized "+countedNoun(d.turns, "turn"))
	}
	if d.pruned > 0 {
		work = append(work, "pruned "+countedNoun(d.pruned, "message"))
	}
	if d.replaced {
		work = append(work, "consolidated the ledger")
	}
	if d.keptAsIs {
		work = append(work, "kept the ledger as written — the rewrite dropped an identifier")
	}
	if len(work) == 0 {
		work = []string{"kept everything verbatim"}
	}
	return "✂ Compaction summary (" + d.tier.String() + ")\n" +
		fmt.Sprintf("   before → after  %s → %s tokens (freed %s)\n", status.ShortTokens(outcome.Before), status.ShortTokens(outcome.After), status.ShortTokens(freed)) +
		fmt.Sprintf("   kept            %d%% verbatim\n", kept) +
		"   work            " + strings.Join(work, ", ")
}
