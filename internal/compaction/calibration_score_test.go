package compaction

import (
	"context"
	"testing"
)

// classifyCorpus asks the live judge about the whole corpus in one batched call,
// on the wire the provider derives. The verdicts carry the probability and the
// choice behind them, which is what lets the sweep re-threshold the answers
// without paying for the corpus again.
func classifyCorpus(t *testing.T, provider, key, goal string, blocks []Block) []Verdict {
	t.Helper()
	spec := SpecFor(provider)
	judge := NewJevJudge(JudgeConfig{
		Enabled:        true,
		Endpoint:       spec.Endpoint,
		BaseURL:        spec.BaseURL,
		Model:          spec.Model,
		APIKey:         key,
		PruneThreshold: DefaultPruneThreshold,
		MaxBlocks:      len(blocks),
	})
	verdicts, err := judge.Classify(context.Background(), goal, blocks)
	if err != nil {
		t.Fatalf("Classify the corpus: %v", err)
	}
	if len(verdicts) != len(blocks) {
		t.Fatalf("verdicts = %d, want one per block", len(verdicts))
	}
	if reporter, ok := judge.(Reporter); ok {
		if answer := reporter.LastAnswer(); answer.Model != "" {
			t.Logf("%s on %s answered by %s · %d input tokens", provider, spec.BaseURL, answer.Model, answer.InputTokens)
		}
	}
	return verdicts
}

// scoreSweep re-thresholds one call's answers across the whole grid, the shipped
// threshold included.
func scoreSweep(verdicts []Verdict, want []Decision) []calibration {
	var out []calibration
	for _, threshold := range calibrationSweep() {
		out = append(out, scoreThreshold(verdicts, want, threshold))
	}
	return out
}

// scoreThreshold is what the gate would have decided at one threshold against the
// labels: the gate itself, not a reimplementation of it, so the sweep scores the
// code that ships.
func scoreThreshold(verdicts []Verdict, want []Decision, threshold float64) calibration {
	var right, pruned, prunedRight, dropped, droppedRight int
	for i, verdict := range verdicts {
		got := decide(map[string]float64{optionPrune: verdict.PruneProb}, verdict.Choice, verdict.Confidence, threshold)
		if got.Decision == want[i] {
			right++
		}
		if got.Decision == Prune {
			pruned++
			if want[i] == Prune {
				prunedRight++
			}
		}
		if want[i] == Prune {
			dropped++
			if got.Decision == Prune {
				droppedRight++
			}
		}
	}
	return calibration{
		threshold:      threshold,
		accuracy:       ratio(right, len(verdicts)),
		prunePrecision: ratio(prunedRight, pruned),
		pruneRecall:    ratio(droppedRight, dropped),
		falsePrune:     pruned - prunedRight,
	}
}

// ratio is a fraction that reads 1 over an empty denominator: a verdict with
// nothing to be right about has nothing to be wrong about either.
func ratio(part, whole int) float64 {
	if whole <= 0 {
		return 1
	}
	return float64(part) / float64(whole)
}
