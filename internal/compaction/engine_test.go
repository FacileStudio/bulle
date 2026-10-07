package compaction

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

type judgeFunc func(ctx context.Context, goal string, blocks []Block) ([]Verdict, error)

func (f judgeFunc) Classify(ctx context.Context, goal string, blocks []Block) ([]Verdict, error) {
	return f(ctx, goal, blocks)
}

func TestEngineThrashTracking(t *testing.T) {
	policy := Policy{Ceiling: 1000}
	engine := NewEngine(policy, nil, nil)
	if engine.Thrashed() || engine.ThrashCount() != 0 {
		t.Fatalf("expected initial thrash count 0, got %d", engine.ThrashCount())
	}
	engine.CheckThrash(500)
	if engine.ThrashCount() != 0 {
		t.Errorf("expected count 0 on size <= trigger, got %d", engine.ThrashCount())
	}
	engine.CheckThrash(1500)
	engine.CheckThrash(1500)
	if engine.Thrashed() || engine.ThrashCount() != 2 {
		t.Errorf("expected count 2 and not thrashed, got count %d", engine.ThrashCount())
	}
	engine.CheckThrash(1500)
	if !engine.Thrashed() || engine.ThrashCount() != 3 {
		t.Errorf("expected thrashed at count 3, got count %d", engine.ThrashCount())
	}
	engine.CheckThrash(800)
	if engine.Thrashed() || engine.ThrashCount() != 0 {
		t.Errorf("expected reset to 0 after landing under, got %d", engine.ThrashCount())
	}
}

func TestEngineEvictionCanLandUnder(t *testing.T) {
	policy := Policy{Ceiling: 10_000, KeepTurns: 1, AnchorMessages: 1}
	engine := NewEngine(policy, nil, nil)
	conv := []nacelle.Message{
		nacelle.UserText("task"),
		nacelle.AssistantText(strings.Repeat("a", 40_000)),
		nacelle.UserText("recent"),
	}
	plan := Plan(conv, policy)
	if !engine.EvictionCanLandUnder(15_000, conv, plan) {
		t.Error("expected eviction to land under ceiling with 40000 byte history")
	}
	tinyConv := []nacelle.Message{
		nacelle.UserText("task"),
		nacelle.AssistantText("hi"),
		nacelle.UserText(strings.Repeat("b", 80_000)),
	}
	tinyPlan := Plan(tinyConv, policy)
	if engine.EvictionCanLandUnder(15_000, tinyConv, tinyPlan) {
		t.Error("expected eviction unable to land under ceiling with tiny history and huge active")
	}
}

func TestEngineOutcomeHelpers(t *testing.T) {
	out := Outcome{Before: 2000, After: 1500, Stage: "judge"}
	if out.FailedAt() != "judge" {
		t.Errorf("FailedAt() = %q, want judge", out.FailedAt())
	}
	if out.Freed() != 500 {
		t.Errorf("Freed() = %d, want 500", out.Freed())
	}
	if out.Installs() {
		t.Error("expected Installs() to be false with empty summary and unjudged")
	}
	out.Summary = "Decisions:\n- kept"
	if !out.Installs() {
		t.Error("expected Installs() to be true with summary")
	}
	judgedOut := Outcome{Judged: true}
	if !judgedOut.Installs() {
		t.Error("expected Installs() to be true when judged and ledger size is 0")
	}
}

func TestEngineRunPassWithoutJudge(t *testing.T) {
	policy := Policy{Ceiling: 500, KeepTurns: 1, AnchorMessages: 1}
	backend := summarizingBackend{answer: "Decisions:\n- keep everything"}
	engine := NewEngine(policy, nil, BackendAgent(backend))
	conv := []nacelle.Message{
		nacelle.UserText("pinned goal"),
		nacelle.AssistantText("history turn 1"),
		nacelle.UserText("history turn 2"),
		nacelle.UserText("active tail"),
	}
	out := engine.RunPass(context.Background(), conv, false)
	if out.Err != nil {
		t.Fatalf("unexpected error: %v", out.Err)
	}
	if out.Summary != "Decisions:\n- keep everything" {
		t.Errorf("summary = %q, want expected summary", out.Summary)
	}
	if len(out.Fold.Ledger) == 0 {
		t.Error("expected all history blocks in ledger when unjudged")
	}
}

func TestEngineRunPassWithJudge(t *testing.T) {
	policy := Policy{Ceiling: 500, KeepTurns: 1, AnchorMessages: 1}
	backend := summarizingBackend{answer: "Decisions:\n- summarized"}
	judge := judgeFunc(func(ctx context.Context, goal string, blocks []Block) ([]Verdict, error) {
		verdicts := make([]Verdict, len(blocks))
		for i := range verdicts {
			verdicts[i] = Verdict{Decision: Keep}
		}
		return verdicts, nil
	})
	engine := NewEngine(policy, judge, BackendAgent(backend))
	conv := []nacelle.Message{
		nacelle.UserText("pinned goal"),
		nacelle.AssistantText("history 1"),
		nacelle.UserText("recent"),
	}
	out := engine.RunPass(context.Background(), conv, true)
	if out.Err != nil {
		t.Fatalf("unexpected error: %v", out.Err)
	}
	if len(out.Fold.Ledger) == 0 {
		t.Error("expected force=true to fold kept blocks into ledger")
	}
}

func TestEngineRunPassJudgeError(t *testing.T) {
	policy := Policy{Ceiling: 500, KeepTurns: 1, AnchorMessages: 1}
	judge := judgeFunc(func(ctx context.Context, goal string, blocks []Block) ([]Verdict, error) {
		return nil, errors.New("judge failure")
	})
	engine := NewEngine(policy, judge, nil)
	conv := []nacelle.Message{
		nacelle.UserText("goal"),
		nacelle.AssistantText("history"),
		nacelle.UserText("active"),
	}
	out := engine.RunPass(context.Background(), conv, false)
	if out.Err == nil || out.Stage != "judge" {
		t.Errorf("expected judge error and stage, got err=%v stage=%q", out.Err, out.Stage)
	}
}

func TestEngineRunPassSummarizerTimeout(t *testing.T) {
	policy := Policy{Ceiling: 500, KeepTurns: 1, AnchorMessages: 1}
	engine := NewEngine(policy, nil, BackendAgent(blockingBackend{}))
	engine.SummarizeTimeout = 10 * time.Millisecond
	conv := []nacelle.Message{
		nacelle.UserText("goal"),
		nacelle.AssistantText("history"),
		nacelle.UserText("active"),
	}
	out := engine.RunPass(context.Background(), conv, false)
	if out.Err == nil || out.Stage != "summary" {
		t.Errorf("expected summary timeout error, got err=%v stage=%q", out.Err, out.Stage)
	}
	if out.Summary != "" {
		t.Errorf("expected empty summary on timeout, got %q", out.Summary)
	}
}
