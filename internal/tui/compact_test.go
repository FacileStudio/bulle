package tui

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// compactAt is the session default, kept as a literal here rather than imported
// from the internal settings package, which package main cannot import. It must
// stay in step with settings.DefaultCompactAt: if that moves, this and the
// session fixtures move with it by hand.
const compactAt int64 = 100_000

func bigConversation() []nacelle.Message {
	result := func(id string, n int) nacelle.ToolResult {
		return nacelle.ToolResult{ID: id, Name: "read", Result: strings.Repeat("x", n)}
	}
	return []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{result("old-1", 40_000), result("small", 100)}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "earlier answer"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{result("old-2", 30_000)}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "middle answer"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{result("recent", 50_000)}},
		{Role: nacelle.RoleAssistant},
	}
}

// The first user turn is pinned as the anchor and the newest turns stay
// verbatim, whatever the ratios do.
func TestPlanPinsTheAnchorAndKeepsTheNewestTurns(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()

	spans := m.plan()

	anchor := compaction.Section(m.conversation, spans, compaction.ZoneAnchor)
	if len(anchor) != 1 || anchor[0].Parts[0].(nacelle.ToolResult).ID != "old-1" {
		t.Errorf("anchor = %v, want the first user turn pinned", anchor)
	}
	if active := compaction.Section(m.conversation, spans, compaction.ZoneActive); len(active) != 3 {
		t.Errorf("active = %d messages, want the newest 3", len(active))
	}
	if _, _, ok := compaction.HistoryRange(spans); !ok {
		t.Error("plan has no history, want the middle eligible for a pass")
	}
}

func TestCompactReportNamesTheWholePass(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	start, end, _ := compaction.HistoryRange(m.plan())
	outcome := compaction.Outcome{
		Before: 125_000,
		After:  90_000,
	}
	done := compacted{evictCut: end - start, turns: 2, kept: len(m.conversation) - end, tier: compaction.Smart}

	line := compactReport(outcome, done)

	for _, want := range []string{"✂", "Compaction summary", "verbatim", "summarized 2 turns", "freed", "(smart)"} {
		if !strings.Contains(line, want) {
			t.Errorf("report = %q, want it to mention %q", line, want)
		}
	}
}

func TestSettleCompactionInstallsASummary(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	anchor := m.conversation[0]
	plan := m.plan()
	outcome := compaction.Outcome{
		Before:  int64(125_000),
		Plan:    plan,
		Fold:    compaction.Fold{Ledger: compaction.Blocks(m.conversation, plan)},
		Tier:    compaction.Smart,
		Summary: "Decisions:\n- done.",
	}

	m.settleCompaction(outcome)

	if m.compacting {
		t.Error("compacting still true after the outcome is installed")
	}
	if !reflect.DeepEqual(m.conversation[0], anchor) {
		t.Errorf("anchor = %+v, want it byte-identical after the pass", m.conversation[0])
	}
	if !installedLedger(m.conversation) {
		t.Errorf("conversation = %v, want a state ledger installed", m.conversation)
	}
	if m.size >= outcome.Before {
		t.Errorf("size = %d, want a summarized pass to shrink below %d", m.size, outcome.Before)
	}
	said := strings.Join(spoken(m), "\n")
	if !strings.Contains(said, "✂ Compaction summary") || !strings.Contains(said, "summarized") {
		t.Errorf("report = %q, want the compaction card naming the summary", said)
	}
}

// The rebuilt conversation alternates roles: the ledger is folded into the turn
// that follows it when both would be the assistant's.
func TestSettleCompactionKeepsRolesAlternating(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	outcome := compaction.Outcome{Before: int64(125_000), Plan: m.plan(), Tier: compaction.Smart, Summary: "Decisions:\n- done."}

	m.settleCompaction(outcome)

	for i := 1; i < len(m.conversation); i++ {
		if m.conversation[i].Role == m.conversation[i-1].Role {
			t.Fatalf("messages %d and %d share role %q: %v", i-1, i, m.conversation[i].Role, m.conversation)
		}
	}
}

func TestCompactPromptFeedsExactlyTheHistory(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	plan := m.plan()
	history := compaction.HistoryMessages(m.conversation, plan)
	fold := compaction.Fold{Ledger: compaction.Blocks(m.conversation, plan)}

	prompt := compaction.Prompt(m.conversation, plan, fold, false)

	if len(prompt) < len(history) || len(prompt) > len(history)+1 {
		t.Fatalf("summarizer prompt = %d messages, want the %d history plus at most the ask", len(prompt), len(history))
	}
	for i := range history {
		if prompt[i].Role != history[i].Role {
			t.Errorf("prompt message %d role = %q, want the history's %q", i, prompt[i].Role, history[i].Role)
		}
	}
	if !promptText(prompt, "older turns") {
		t.Error("prompt = no compact ask, want the request appended")
	}
	for i := 1; i < len(prompt); i++ {
		if prompt[i].Role == prompt[i-1].Role {
			t.Errorf("messages %d and %d share role %q, want the ask folded in rather than two user turns", i-1, i, prompt[i].Role)
		}
	}
}

func promptText(messages []nacelle.Message, want string) bool {
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if text, ok := part.(nacelle.Text); ok && strings.Contains(text.Text, want) {
				return true
			}
		}
	}
	return false
}

// A judge failure is named as the judge's and degrades to the mask: nothing is
// pruned, the conversation stands, and the session continues.
func TestSettleCompactionReportsAJudgeFailure(t *testing.T) {
	m := sized()
	m.conversation = bigConversation()
	m.size = compactAt + 25_000
	outcome := compaction.Outcome{
		Before: m.size,
		Plan:   m.plan(),
		Tier:   compaction.Smart,
		Judged: true,
		Stage:  "judge",
		Err:    errors.New("typesafe: http 429"),
	}

	m.settleCompaction(outcome)

	if said := strings.Join(spoken(m), " "); !strings.Contains(said, "compaction judge failed") {
		t.Errorf("said = %q, want the judge failure named", said)
	}
	if len(m.conversation) != len(bigConversation()) {
		t.Errorf("conversation = %d messages, want the failed pass to prune nothing", len(m.conversation))
	}
}

func TestCompactPromptScopesTheSummaryToItsChunk(t *testing.T) {
	if !strings.Contains(compaction.SystemPrompt, "only the turns shown to you") {
		t.Errorf("compactSystem = %q, want an explicit chunk-scoping rule", compaction.SystemPrompt)
	}
	if !strings.Contains(compaction.SystemPrompt, "not part of this request") && !strings.Contains(compaction.CompactAsk, "not part of this request") {
		t.Errorf("prompts = %q / %q, want an explicit 'not part of this request' boundary", compaction.SystemPrompt, compaction.CompactAsk)
	}
	for _, section := range []string{"Decisions", "Constraints", "Plan", "State", "Artifacts", "Ruled out", "Open questions"} {
		if !strings.Contains(compaction.SystemPrompt, section) {
			t.Errorf("compactSystem missing the %q section of the schema", section)
		}
	}
	if !strings.Contains(compaction.SystemPrompt, "Never invent facts") {
		t.Errorf("compactSystem = %q, want a no-invention rule", compaction.SystemPrompt)
	}
}

func installedLedger(messages []nacelle.Message) bool {
	return slices.ContainsFunc(messages, compaction.IsLedger)
}
