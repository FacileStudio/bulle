package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/compaction"
)

const thrashLimit = compaction.DefaultThrashLimit

func (m *Model) evictionCanLandUnder(start, end int) bool {
	return m.size-estTokens(compaction.Bytes(m.conversation[start:end]))+compactMaxTokens <= m.compactAt
}

func (m *Model) maskOnlyPass(plan []compaction.Span) tea.Cmd {
	before := m.size
	tier := m.engine().Policy.Tier(before)
	stats := m.maskHistory(plan)
	start, end, _ := compaction.HistoryRange(plan)
	report := compaction.Outcome{
		Before: before,
		After:  m.size,
		Plan:   plan,
		Tier:   tier,
	}
	m.last = compacted{evictCut: end - start, kept: len(m.conversation) - end, results: stats.Results, tier: tier}
	m.say(fromCompact, compactReport(report, m.last)+"\n   cost sits in the kept tail — compaction protects the newest turns; /clear or read in chunks")
	m.checkThrash()
	return nil
}

func (m *Model) softPass() tea.Cmd {
	plan := m.plan()
	if compaction.DroppableBytes(m.conversation, plan) < compaction.MinCleared {
		return nil
	}
	before := m.size
	stats := m.maskHistory(plan)
	if stats.Results == 0 {
		return nil
	}
	start, end, _ := compaction.HistoryRange(plan)
	report := compaction.Outcome{
		Before: before,
		After:  m.size,
		Plan:   plan,
		Tier:   compaction.Soft,
	}
	m.last = compacted{
		evictCut: end - start,
		kept:     len(m.conversation) - end,
		results:  stats.Results,
		tier:     compaction.Soft,
	}
	m.say(fromCompact, compactReport(report, m.last))
	return nil
}

func (m *Model) compactTiered(ctx context.Context) tea.Cmd {
	switch m.engine().Policy.Tier(m.size) {
	case compaction.Soft:
		return m.softPass()
	case compaction.Smart:
		return m.beginCompaction(ctx, false)
	default:
		return nil
	}
}

func (m *Model) thrashed() bool {
	return m.engine().Thrashed()
}

func (m *Model) checkThrash() {
	engine := m.engine()
	if m.compactAt > 0 && engine.CheckThrash(m.size) && engine.ThrashCount() == compaction.DefaultThrashLimit {
		m.say(fromClient, "compaction keeps leaving the context over the threshold — one very large result is likeliest; /clear, read in chunks, then /compact")
	}
}
