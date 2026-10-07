package compaction

import "testing"

// reportVerdicts prints one line per block, which is what a human tuning the
// criteria actually reads: the blocks the model disagreed with, and how sure it
// was. A run where every confidence sits in one low band is a question that has
// no confident answer to give, and no threshold can be tuned around that.
func reportVerdicts(t *testing.T, blocks []Block, verdicts []Verdict, want []Decision) {
	t.Helper()
	for i, verdict := range verdicts {
		got := decide(map[string]float64{optionPrune: verdict.PruneProb}, verdict.Choice, verdict.Confidence, DefaultPruneThreshold)
		mark := " "
		if got.Decision != want[i] {
			mark = "!"
		}
		t.Logf("%s %-24s want %-6s got %-6s prune %.2f conf %.2f",
			mark, blocks[i].Key, want[i], got.Decision, verdict.PruneProb, verdict.Confidence)
	}
}

// reportCalibration prints the sweep, marking the shipped threshold. Recall
// matters more than precision here in one direction only: a missed prune costs a
// little context, and a false prune costs a fact nothing can recover, so the
// column to read first is the false-prune count.
func reportCalibration(t *testing.T, scores []calibration) {
	t.Helper()
	t.Log("threshold  accuracy  prune recall  prune precision  false prunes")
	for _, score := range scores {
		mark := " "
		if score.threshold == DefaultPruneThreshold {
			mark = "*"
		}
		t.Logf("%s%.2f       %3.0f%%      %3.0f%%           %3.0f%%             %d",
			mark, score.threshold, score.accuracy*100, score.pruneRecall*100, score.prunePrecision*100, score.falsePrune)
	}
}

// reportConfidenceBands is the "where confidence and accuracy diverge" half of
// the guidance: verdicts bucketed by the confidence the model reported, with the
// share it got right inside each. A band whose accuracy sits well under its own
// confidence is where the calibrated number is not calibrating, and where a
// threshold should not be trusted.
func reportConfidenceBands(t *testing.T, verdicts []Verdict, want []Decision) {
	t.Helper()
	type band struct{ count, right int }
	bands := map[int]*band{}
	for i, verdict := range verdicts {
		bucket := min(int(verdict.Confidence*10), 10)
		if bands[bucket] == nil {
			bands[bucket] = &band{}
		}
		bands[bucket].count++
		got := decide(map[string]float64{optionPrune: verdict.PruneProb}, verdict.Choice, verdict.Confidence, DefaultPruneThreshold)
		if got.Decision == want[i] {
			bands[bucket].right++
		}
	}
	for bucket := range 11 {
		if bands[bucket] == nil {
			continue
		}
		t.Logf("  confidence %.1f-%.1f · %d blocks · %.0f%% right",
			float64(bucket)/10, float64(bucket+1)/10, bands[bucket].count, ratio(bands[bucket].right, bands[bucket].count)*100)
	}
}

// assertCalibration holds the shipped threshold to two things: no block labeled
// keep may be pruned — a false prune is the one error the conversation cannot
// recover from, and the asymmetric gate exists to make it impossible rather than
// unlikely — and the corpus accuracy has to clear the provider's floor, which is
// what catches a threshold that has come loose from the model behind it.
func assertCalibration(t *testing.T, floor float64, scores []calibration) {
	t.Helper()
	for _, score := range scores {
		if score.threshold != DefaultPruneThreshold {
			continue
		}
		if score.falsePrune > 0 {
			t.Errorf("%d block(s) labeled keep were pruned at the shipped threshold %v: the gate is letting the one destructive verdict through",
				score.falsePrune, DefaultPruneThreshold)
		}
		if score.accuracy < floor {
			t.Errorf("accuracy at the shipped threshold = %.0f%%, under the %.0f%% floor — the corpus or the thresholds have moved",
				score.accuracy*100, floor*100)
		}
	}
}
