package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/compaction"
)

// compactFinished says the compaction channel closed and no outcome arrived.
type compactFinished struct{}

// compactMaxTokens is the ceiling a compaction summary is asked to stay under.
// Small on purpose: the ledger replaces a large history with a stub, so a long
// summary would buy almost nothing. It is the ledger's own budget as well — a
// body larger than one summary is carrying more than a compression should and is
// what sends the next pass to consolidation — so the two are one number, aliased
// rather than repeated, and they cannot drift apart.
const compactMaxTokens = compaction.MaxLedgerTokens

// resolvedPolicy fills the ladder and the tail a SessionConfig left out, so a
// caller that only ever set the ceiling still gets the shipped defaults instead
// of a zero ratio that would never fire, or a zeroed tail that would pin
// nothing. The window, the reserve and the ceiling are taken as given: the
// session config is the layer that already resolved compact_at against them.
func resolvedPolicy(base compaction.Policy) compaction.Policy {
	policy := base
	if policy.Ratios == (compaction.Ratios{}) {
		policy.Ratios = compaction.Ratios{
			Soft:  compaction.DefaultSoftRatio,
			Smart: compaction.DefaultSmartRatio,
		}
	}
	if policy.KeepTurns <= 0 {
		policy.KeepTurns = compaction.DefaultKeepTurns
	}
	if policy.KeepTokens <= 0 {
		policy.KeepTokens = compaction.DefaultKeepTokens
	}
	if policy.AnchorMessages <= 0 {
		policy.AnchorMessages = compaction.DefaultAnchorMessages
	}
	return policy
}

// plan is the current partition of the conversation into anchor, ledger,
// history and active spans.
func (m *Model) plan() []compaction.Span {
	return compaction.Plan(m.conversation, m.policy)
}

// beginCompaction runs one summarizing pass: it feeds the history zone to the
// tool-free summarizer and installs the result as the ledger. It is called from
// the tiered trigger once the measured size has crossed the soft band, from the
// manual /compact command, and from the pre-send path. It is where the
// running-tool row and the purple status come from: a pass is an LLM call now,
// so it must not block the update loop. When the history is too small to
// plausibly land the conversation under the ceiling, the pass skips the
// summarizer and tombstones what little old turns hold instead — see
// evictionCanLandUnder/maskOnlyPass in compact_light.go.
//
// force is overflow recovery's one lever: a run the provider refused for length
// is by definition past whatever the ladder last measured, so the retry asks for
// the whole history folded outright rather than re-deriving a tier from a size
// that was already wrong once.
func (m *Model) beginCompaction(ctx context.Context, force bool) tea.Cmd {
	plan := m.plan()
	start, end, ok := compaction.HistoryRange(plan)
	if !ok || m.compacting {
		return nil
	}
	if !m.evictionCanLandUnder(start, end) {
		return m.maskOnlyPass(plan)
	}

	m.compacting = true
	m.compactBegan = time.Now()

	resultsChan := make(chan compactOutcome)
	m.run.compactChan = resultsChan

	go runCompaction(ctx, resultsChan, m.pass(plan, m.policy.Tier(m.size), force))
	return tea.Batch(waitForCompact(resultsChan), m.spin.Tick)
}

// runCompaction is the pass's own goroutine. It asks the backend for a summary
// of the raw history zone and sends back just that result; it never mutates the
// conversation, and it never touches the Model at all — everything it reads
// arrives on the compactPass the update loop snapshotted for it.
//
// A consolidating pass folds the history whether or not the judge tagged any of
// it, which is the same lever the derived force pulls. The rewrite such a pass asks
// for is legitimate only when it is measured against turns no earlier pass
// compressed (I3b), and those turns are what carry the earlier ledger into the
// ask: with the fold empty there is no turn to carry it, so compactPrompt drops
// the ledger, the consolidating addendum never reaches the model, and a summary
// of nothing comes back to replace a body that was over budget. Forcing here is
// what makes the trigger the ledger's own size rather than the judge's verdicts,
// which is the one reading under which consolidation can actually happen.
//
// The summarizer runs inside a deadline set by summarizeInto, so a wedged
// backend cannot hold the session at "compacting" forever: whichever way the
// stream winds down once the deadline fires, the outcome still arrives and
// the pass falls back to the mask.
func runCompaction(ctx context.Context, results chan compactOutcome, pass compactPass) {
	defer close(results)
	outcome := compactOutcome{
		before:      pass.size,
		plan:        pass.plan,
		tier:        pass.tier,
		judged:      pass.judge != nil,
		consolidate: pass.consolidate,
	}

	judgeCtx, cancel := context.WithTimeout(ctx, compactJudgeTimeout)
	fold, err := compaction.Classify(judgeCtx, pass.conv, pass.plan, compaction.JudgeRequest{
		Goal: compaction.JudgeGoal(pass.conv, pass.plan),
	}, pass.judge)
	cancel()
	if err != nil {
		outcome.err, outcome.stage = err, "judge"
		results <- outcome
		return
	}
	if pass.force || pass.consolidate || !compaction.LandsUnder(pass.conv, pass.plan, fold, pass.trigger) {
		fold = fold.Forced()
	}
	outcome.fold = fold

	if len(fold.Ledger) > 0 && pass.agent != nil {
		summary, err := summarizeInto(ctx, pass.agent, compactPrompt(pass.conv, pass.plan, fold, pass.consolidate))
		if err != nil {
			outcome.err, outcome.stage = err, "summary"
		} else {
			outcome.summary = summary
		}
	}

	results <- outcome
}

// settleCompaction installs a finished pass and starts the run that was
// waiting on the freed context, or on the idle path sends the lines the reader
// typed during the pass. A summary rebuilds the conversation as anchor + ledger
// + active, or the tombstone stands, and a pass never grows the conversation.
// Delivery lives here, not chained in settle, where a detached sequence would
// race this install.
func (m *Model) settleCompaction(outcome compactOutcome) tea.Cmd {
	m.compacting = false
	m.run.compactChan = nil

	if outcome.err != nil {
		m.say(fromCompact, "compaction "+outcome.failedAt()+" failed · "+outcome.err.Error()+maskNote(m.applyMaskFallback(outcome)))
	} else if outcome.installs() {
		m.installFold(outcome)
	} else {
		m.say(fromCompact, "compaction summary came back empty"+maskNote(m.applyMaskFallback(outcome)))
	}

	m.checkThrash()

	if m.run.busy && m.agent != nil {
		return m.startRun(m.run.bgCtx)
	}
	return m.deliver()
}

// installFold rebuilds the conversation around the ledger and the surviving
// blocks and reports the pass. The after size is the authoritative before size
// adjusted by the two estimates Apply measured, so the report stays anchored to
// the backend's own count while the pass's own arithmetic never grows it.
//
// A refused rebuild leaves everything where it was and says so. It is the one
// judged shape where folding costs more than it saves — a tiny turn folded into a
// ledger longer than the turn was — and installing it would grow the context the
// pass exists to shrink. The conversation and the size are already correct, so
// the pass only has to be honest about having bought nothing.
//
// A stale plan is the other way out, and the more dangerous one: the pass measured
// a conversation that is no longer the one in the field, so its spans bound
// nothing. Apply refuses it and this reports it, rather than folding a plan into
// whatever happens to be sitting at those indices.
func (m *Model) installFold(outcome compactOutcome) {
	start, end, _ := compaction.HistoryRange(outcome.plan)
	kept := len(compaction.Section(m.conversation, outcome.plan, compaction.ZoneActive))

	conv, stats := compaction.Apply(m.conversation, outcome.plan, outcome.summary, outcome.fold.Survives, outcome.consolidate)
	if stats.Stale {
		m.say(fromCompact, "the conversation changed while the pass ran — nothing was folded")
		return
	}
	if stats.Refused {
		m.last = compacted{tier: outcome.tier}
		m.say(fromCompact, "context unchanged — the ledger would have outweighed the turns it folds")
		return
	}

	m.conversation = conv
	m.size = max(outcome.before-stats.Before+stats.After, 0)
	outcome.after = m.size
	outcome.done = compacted{
		evictCut: end - start,
		turns:    outcome.fold.LedgerSize(),
		pruned:   outcome.fold.PrunedSize(),
		kept:     kept,
		tier:     outcome.tier,
		replaced: stats.LedgerReplaced,
		keptAsIs: stats.LedgerKept,
	}
	m.last = outcome.done
	m.say(fromCompact, compactReport(outcome))
}

// waitForCompact takes exactly one outcome and re-arms itself from Update,
// the same contract waitFor holds for run results.
func waitForCompact(results <-chan compactOutcome) tea.Cmd {
	return func() tea.Msg {
		next, open := <-results
		if !open {
			return compactFinished{}
		}
		return next
	}
}
