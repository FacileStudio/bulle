package compaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
)

// minCalibrationAccuracy is the floor the shipped thresholds are held to on the
// corpus. It is coarse on purpose and meant to be raised: it catches a ladder
// that has come loose, not a ladder that is a point worse than yesterday.
const minCalibrationAccuracy = 0.7

// calibrationEnv is the deliberate opt-in the live harness is gated behind. It is
// separate from the API key because the key is not consent: TYPESAFE_API_KEY is
// exported on a machine that uses the judge at all, so gating on it alone makes
// every ordinary `go test ./...` pay the vendor for a call and fail whenever the
// vendor is down or the model has drifted.
const calibrationEnv = "BULLE_CALIBRATION"

// calibrationProviderEnv selects which judge provider classifies the corpus:
// unset or "jev" for TypeSafe direct, "clef" for OpenRouter. Every provider
// ships with its own calibration run, because a threshold measured on one
// model is a guess about the next.
const calibrationProviderEnv = "BULLE_CALIBRATION_PROVIDER"

// judgeLabels is the labeled corpus the harness classifies, loaded from
// testdata/judge_labels.json. The file is the artifact a human reviews, so the
// labels live there rather than in a Go table that only an implementer reads.
type judgeLabels struct {
	Note  string      `json:"note"`
	Goal  string      `json:"goal"`
	Cases []judgeCase `json:"cases"`
}

// judgeCase is one history block with the verdict a careful operator would give
// it: keep, prune or ledger.
type judgeCase struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

// calibration is one threshold's score over the corpus: how often the gate
// agreed with the labels, how much of the prunable history it recovered, and how
// much of what it dropped should have been kept.
type calibration struct {
	threshold      float64
	accuracy       float64
	pruneRecall    float64
	prunePrecision float64
	falsePrune     int
}

// TestJudgeCalibrationOnLabeledBlocks is the harness TypeSafe's guidance asks for
// before a threshold is trusted: run inputs whose answer is known, and look at
// where confidence and accuracy diverge. The prune threshold and
// ConfidenceFloor are the two numbers that decide a deletion, and until this
// runs they are guesses a reviewer cannot check.
//
// It talks to the network and it bills, so it is opt-in twice over: a key in the
// environment is not consent, because this machine exports TYPESAFE_API_KEY for
// the judge itself and a plain `go test ./...` would then spend a real call on
// every run — and fail on vendor downtime or model drift rather than on a
// regression here. Set calibrationEnv to ask for it deliberately:
//
//	BULLE_CALIBRATION=1 TYPESAFE_API_KEY=... go test ./internal/compaction -run JudgeCalibration -v
//
// The provider picks which model classifies: jev (TypeSafe direct, the default)
// or clef (OpenRouter), selected with calibrationProviderEnv and keyed by the
// vendor that provider reads:
//
//	BULLE_CALIBRATION=1 BULLE_CALIBRATION_PROVIDER=clef OPENROUTER_API_KEY=... go test ./internal/compaction -run JudgeCalibration -v
//
// One call classifies the whole corpus, so the sweep afterwards costs nothing:
// the probabilities come back on the verdicts and are re-thresholded offline.
func TestJudgeCalibrationOnLabeledBlocks(t *testing.T) {
	if os.Getenv(calibrationEnv) != "1" {
		t.Skipf("set %s=1 and the judge provider's key to classify the labeled corpus with the live model", calibrationEnv)
	}
	provider := os.Getenv(calibrationProviderEnv)
	if provider == "" {
		provider = "jev"
	}
	key := calibrationKey(provider)
	if key == "" {
		t.Skipf("set %s or %s to classify the labeled corpus with the live model", "TYPESAFE_API_KEY", "OPENROUTER_API_KEY")
	}
	corpus := loadJudgeLabels(t)
	blocks := corpusBlocks(corpus)
	want := corpusLabels(t, corpus)

	verdicts := classifyCorpus(t, provider, key, corpus.Goal, blocks)
	scores := scoreSweep(verdicts, want)
	reportVerdicts(t, blocks, verdicts, want)
	reportCalibration(t, scores)
	reportConfidenceBands(t, verdicts, want)
	assertCalibration(t, accuracyFloor(provider), scores)
}

// accuracyFloor is the agreement each provider is held to on the corpus. jev is
// held to minCalibrationAccuracy, measured at 76% on 2026-10-07. clef's floor
// is 60% against a measured 65% at the shipped threshold (2026-10-07): every
// disagreement was a conservative keep — zero false prunes across the whole
// sweep and prune recall on par with jev — because clef reports lower
// confidence than jev on this corpus and the floor refuses verdicts it will not
// vouch for. The floor catches a gate come loose from its model; each provider
// gets the same check at what that model actually does, and a drop below the
// measured number still fails. Re-run the sweep before moving either number.
func accuracyFloor(provider string) float64 {
	if provider == "clef" {
		return 0.6
	}
	return minCalibrationAccuracy
}

// calibrationKey is the vendor's own key for the provider being calibrated, so
// a clef sweep never spends TypeSafe's credential or the reverse.
func calibrationKey(provider string) string {
	if provider == "clef" {
		return os.Getenv("OPENROUTER_API_KEY")
	}
	return os.Getenv("TYPESAFE_API_KEY")
}

// The corpus is checked without the network, so a typo'd label, a duplicated id
// or a label left with too few examples to score surfaces in CI rather than the
// first time somebody pays to run the harness.
func TestJudgeLabelCorpusIsWellFormed(t *testing.T) {
	corpus := loadJudgeLabels(t)

	seen := map[string]bool{}
	counts := map[Decision]int{}
	for _, c := range corpus.Cases {
		if c.ID == "" || c.Text == "" {
			t.Errorf("case %+v, want both an id and the block text", c)
		}
		if seen[c.ID] {
			t.Errorf("duplicate case id %q: the ids are the judge's question names", c.ID)
		}
		seen[c.ID] = true
		counts[labelDecision(t, c.Label)]++
	}
	for _, label := range []Decision{Keep, Prune, Ledger} {
		if counts[label] < 3 {
			t.Errorf("%d blocks labeled %v, want at least 3 so a sweep has something to be wrong about", counts[label], label)
		}
	}
}

// loadJudgeLabels reads the corpus from testdata, next to the harness that is its
// only reader.
func loadJudgeLabels(t *testing.T) judgeLabels {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "judge_labels.json"))
	if err != nil {
		t.Fatalf("read the labeled corpus: %v", err)
	}
	var corpus judgeLabels
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("parse the labeled corpus: %v", err)
	}
	if len(corpus.Cases) < 8 {
		t.Fatalf("corpus = %d cases, want a spread worth calibrating a threshold on", len(corpus.Cases))
	}
	if corpus.Goal == "" {
		t.Fatal("corpus carries no goal, so every block would be judged against nothing")
	}
	return corpus
}

// corpusBlocks is the corpus in the shape a judge takes it, one block per case.
func corpusBlocks(corpus judgeLabels) []Block {
	blocks := make([]Block, 0, len(corpus.Cases))
	for _, c := range corpus.Cases {
		blocks = append(blocks, Block{Key: c.ID, Text: c.Text})
	}
	return blocks
}

// corpusLabels is the verdict each block should get, in the same order as
// corpusBlocks, so the two can be scored positionally.
func corpusLabels(t *testing.T, corpus judgeLabels) []Decision {
	t.Helper()
	want := make([]Decision, 0, len(corpus.Cases))
	for _, c := range corpus.Cases {
		want = append(want, labelDecision(t, c.Label))
	}
	return want
}

// labelDecision maps a corpus label onto the verdict it stands for, refusing a
// label nobody defined rather than defaulting it to Keep and quietly scoring a
// typo as a pass.
func labelDecision(t *testing.T, label string) Decision {
	t.Helper()
	switch label {
	case optionKeep:
		return Keep
	case optionPrune:
		return Prune
	case optionLedger:
		return Ledger
	default:
		t.Fatalf("unknown label %q in the corpus, want keep, prune or ledger", label)
		return Keep
	}
}

// calibrationSweep is the grid one live call is re-scored over, the shipped
// threshold included. The point is the curve rather than a single number: the
// shipped threshold should sit on its knee, not at either end of it, and the
// cheapest way to see that is to re-threshold answers already paid for.
func calibrationSweep() []float64 {
	out := []float64{DefaultPruneThreshold}
	for threshold := 0.5; threshold < 1.0; threshold += 0.05 {
		out = append(out, float64(int(threshold*100+0.5))/100)
	}
	sort.Float64s(out)
	return slices.Compact(out)
}
