package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

type compactFinished struct{}

const compactMaxTokens = compaction.MaxLedgerTokens

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

func (m *Model) plan() []compaction.Span {
	return compaction.Plan(m.conversation, m.compactor.Policy)
}

func (m *Model) engine() *compaction.Engine {
	if m.compactor == nil {
		m.compactor = compaction.NewEngine(compaction.Policy{}, nil, nil)
	}
	if m.compactor.Builder == nil && m.agent != nil {
		m.compactor.Builder = func() (*nacelle.Agent, error) {
			if m.agent == nil {
				return nil, nil
			}
			return compaction.BackendAgent(m.agent.Backend())()
		}
	}
	return m.compactor
}

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

	resultsChan := make(chan compaction.Outcome, 1)
	m.run.compactChan = resultsChan

	engine := m.engine()
	conv := compaction.Snapshot(m.conversation)
	size := m.size
	go func() {
		defer close(resultsChan)
		resultsChan <- engine.RunPassWithSize(ctx, conv, size, force)
	}()
	return tea.Batch(waitForCompact(resultsChan), m.spin.Tick)
}

func (m *Model) settleCompaction(outcome compaction.Outcome) tea.Cmd {
	m.compacting = false
	m.run.compactChan = nil

	if outcome.Err != nil {
		m.say(fromCompact, "compaction "+outcome.FailedAt()+" failed · "+outcome.Err.Error()+maskNote(m.applyMaskFallback(outcome)))
	} else if outcome.Installs() {
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

func (m *Model) installFold(outcome compaction.Outcome) {
	start, end, _ := compaction.HistoryRange(outcome.Plan)
	kept := len(compaction.Section(m.conversation, outcome.Plan, compaction.ZoneActive))

	conv, stats := outcome.Apply(m.conversation)
	if stats.Stale {
		m.say(fromCompact, "the conversation changed while the pass ran — nothing was folded")
		return
	}
	if stats.Refused {
		m.last = compacted{tier: outcome.Tier}
		m.say(fromCompact, "context unchanged — the ledger would have outweighed the turns it folds")
		return
	}

	m.conversation = conv
	m.size = outcome.After
	m.last = compacted{
		evictCut: end - start,
		turns:    outcome.Fold.LedgerSize(),
		pruned:   outcome.Fold.PrunedSize(),
		kept:     kept,
		tier:     outcome.Tier,
		replaced: stats.LedgerReplaced,
		keptAsIs: stats.LedgerKept,
	}
	m.say(fromCompact, compactReport(outcome, m.last))
}

func waitForCompact(results <-chan compaction.Outcome) tea.Cmd {
	return func() tea.Msg {
		next, open := <-results
		if !open {
			return compactFinished{}
		}
		return next
	}
}
