package compaction

import (
	"context"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// RunPass executes a compaction pass measuring token size from the conversation.
func (e *Engine) RunPass(ctx context.Context, conv []nacelle.Message, force bool) Outcome {
	return e.RunPassWithSize(ctx, conv, EstTokens(Bytes(conv)), force)
}

// RunPassWithSize executes a compaction pass with an explicit starting token size.
func (e *Engine) RunPassWithSize(ctx context.Context, conv []nacelle.Message, size int64, force bool) Outcome {
	safeConv := Snapshot(conv)
	plan := Plan(safeConv, e.Policy)
	consolidate := LedgerOverBudget(LedgerText(safeConv, plan))
	outcome := Outcome{
		Before:      size,
		Plan:        plan,
		Tier:        e.Policy.Tier(size),
		Judged:      e.Judge != nil,
		Consolidate: consolidate,
	}
	fold, err := e.classifyFold(ctx, safeConv, plan)
	if err != nil {
		outcome.Fold = fold
		outcome.Err = err
		outcome.Stage = "judge"
		return outcome
	}
	if force || consolidate || !LandsUnder(safeConv, plan, fold, e.Policy.Trigger()) {
		fold = fold.Forced()
	}
	outcome.Fold = fold
	summary, err := e.summarizeFold(ctx, safeConv, plan, fold, consolidate)
	if err != nil {
		outcome.Err = err
		outcome.Stage = "summary"
		return outcome
	}
	outcome.Summary = summary
	return outcome
}

func (e *Engine) classifyFold(ctx context.Context, conv []nacelle.Message, plan []Span) (Fold, error) {
	judgeCtx, cancel := context.WithTimeout(ctx, e.judgeTimeout())
	defer cancel()
	req := JudgeRequest{Goal: JudgeGoal(conv, plan)}
	return Classify(judgeCtx, conv, plan, req, e.Judge)
}

func (e *Engine) summarizeFold(ctx context.Context, conv []nacelle.Message, plan []Span, fold Fold, consolidate bool) (string, error) {
	if len(fold.Ledger) == 0 || e.Builder == nil {
		return "", nil
	}
	agent, err := e.Builder()
	if err != nil {
		return "", err
	}
	if agent == nil {
		return "", nil
	}
	asks := Prompt(conv, plan, fold, consolidate)
	return summarizeInto(ctx, agent, asks, e.summarizeTimeout())
}

func summarizeInto(parent context.Context, agent *nacelle.Agent, asks []nacelle.Message, timeout time.Duration) (string, error) {
	local, cancel := context.WithTimeout(parent, timeout)
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

func (e *Engine) judgeTimeout() time.Duration {
	if e.JudgeTimeout > 0 {
		return e.JudgeTimeout
	}
	return DefaultJudgeTimeout
}

func (e *Engine) summarizeTimeout() time.Duration {
	if e.SummarizeTimeout > 0 {
		return e.SummarizeTimeout
	}
	return DefaultSummarizeTimeout
}
