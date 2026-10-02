package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// compacted holds one compaction pass's numbers, so the report and the tests
// that check it share one shape rather than a bag of named arguments. evictCut
// and kept are the history and active sizes the pass measured, which is what the
// "kept N% verbatim" line is drawn from; results, turns and pruned are what it
// actually did.
type compacted struct {
	evictCut int
	turns    int
	kept     int
	pruned   int
	results  int
	tier     compaction.Tier
	// replaced and keptAsIs are what happened to the ledger body itself, which
	// is the one part of a pass whose size is not the mask's or the fold's: a
	// pass that consolidated a ledger that had outgrown its budget, and a
	// consolidating pass that was refused because the rewrite had dropped an
	// identifier. Both are worth a line, because they are the ledger's economy
	// and nothing else in the report speaks to it.
	replaced bool
	keptAsIs bool
}

// compactOutcome is what a compaction pass sends back to the update loop. On the
// wire it carries the size before, the plan the pass ran against, the tier it
// selected and the summary the backend produced, plus any error from the
// summarizer call. The install and report functions fill in the after size, the
// rebuilt conversation and the tallies on the UI thread, so the goroutine never
// races a list it is also reading.
type compactOutcome struct {
	before      int64
	after       int64
	done        compacted
	plan        []compaction.Span
	fold        compaction.Fold
	tier        compaction.Tier
	summary     string
	judged      bool
	consolidate bool
	stage       string
	err         error
}

// failedAt names which half of a pass failed, for the notice: the judge that
// classifies, or the summarizer that writes the ledger. An outcome built without
// a stage — a test's — reads as the summary, the half that has always existed.
func (o compactOutcome) failedAt() string {
	if o.stage != "" {
		return o.stage
	}
	return "summary"
}

// installs reports whether a finished pass may replace the conversation at all.
// An empty summary is a failure whenever the judge tagged turns for the ledger:
// the fold would drop those turns and install no ledger, so the reader would lose
// silently what the pass was meant to keep in compressed form. It is the same
// fallback the unjudged path already takes, reached by the same test. A judged
// pass that tagged nothing has nothing to summarize and still installs the keeps
// and the prunes it decided on.
func (o compactOutcome) installs() bool {
	return o.summary != "" || (o.judged && o.fold.LedgerSize() == 0)
}

// compactReport is how the client says a pass went: the context size it carried
// before and after, the share kept verbatim, what the mask and the summary did,
// and the tier that ran. A short block, so it reads as a visible milestone in the
// transcript rather than a one-word aside.
func compactReport(outcome compactOutcome) string {
	d := outcome.done
	freed := outcome.before - outcome.after
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
		fmt.Sprintf("   before → after  %s → %s tokens (freed %s)\n", shortTokens(outcome.before), shortTokens(outcome.after), shortTokens(freed)) +
		fmt.Sprintf("   kept            %d%% verbatim\n", kept) +
		"   work            " + strings.Join(work, ", ")
}

// compactTimeout bounds one summarizer call. A backend that hangs would
// otherwise hold the session at \"compacting\" forever, because settleCompaction
// runs only when the outcome arrives. A pass past the deadline is a failure:
// its partial summary is dropped and the mask fallback runs, which still
// frees the bulky tool output — so a spurious timeout costs an attempt, never
// correctness. Generous, because the pass replaces a large chunk (possibly
// hundreds of KB) with a short summary.
const compactTimeout = 120 * time.Second

// compactJudgeTimeout bounds the classification half of a pass. The judge is a
// short typed call and the client retries internally, so this is a backstop
// against a wedged endpoint, not a budget: the deadline is read like the
// summarizer's — a pass past it fails and the mask takes over.
const compactJudgeTimeout = 30 * time.Second

// summarizeInto runs one bounded summarizer call. The agent streams the chunk
// on a child context carrying a deadline, so a hung backend frees the session
// rather than holding it. A stream error, or a context whose deadline fired
// while the transport wound down cleanly, both come back as the error and the
// partial text is discarded: a transport that honours cancellation by stopping
// reports nothing through the stream, so the deadline must be read off the
// context itself.
func summarizeInto(parent context.Context, agent *nacelle.Agent, asks []nacelle.Message) (string, error) {
	local, cancel := context.WithTimeout(parent, compactTimeout)
	defer cancel()

	var b strings.Builder
	for event, err := range agent.Stream(local, asks) {
		if err != nil {
			return "", err
		}
		if event.Kind == nacelle.KindText {
			b.WriteString(event.Text)
		}
	}
	if localErr := local.Err(); localErr != nil {
		return "", localErr
	}
	return strings.TrimSpace(b.String()), nil
}
