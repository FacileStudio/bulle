package compaction

import (
	"context"

	"github.com/FacileStudio/bulle/internal/jev"
)

// Decision is what the judge wants done with one history block.
type Decision uint8

const (
	// Keep leaves the block in the conversation verbatim.
	Keep Decision = iota
	// Prune drops the whole atomic block. It is the only destructive verdict,
	// so it is the only one the confidence gate guards.
	Prune
	// Ledger folds the block's facts into the state ledger, compressed by the
	// summarizer, and drops the block itself.
	Ledger
)

// String names a decision the way a report or a test reads it.
func (d Decision) String() string {
	switch d {
	case Prune:
		return "prune"
	case Ledger:
		return "ledger"
	default:
		return "keep"
	}
}

// Verdict is one block's classification, carrying the numbers behind it so a
// caller can log or re-threshold without asking again.
type Verdict struct {
	Decision Decision
	// Choice is the option the judge picked, kept beside the probabilities and
	// not only folded into Decision: re-thresholding needs it to tell a Ledger
	// verdict from a Keep, and the calibration harness sweeps the prune threshold
	// over one call's answers rather than paying for the corpus again.
	Choice     string
	PruneProb  float64
	Confidence float64
}

// Judge classifies history blocks against the task the session started from.
// It decides keep, prune or ledger per block and writes no prose: the ledger is
// the summarizer's job, fed only the blocks this tags.
type Judge interface {
	Classify(ctx context.Context, goal string, blocks []Block) ([]Verdict, error)
}

// Answer is what a judge's most recent call came back from: the versioned id of
// the model that answered it, and what it billed. TypeSafe ships a drifting
// `jev-latest` alias, reports the id that actually answered in every response,
// and tells callers to log it and pin it once thresholds have been tuned against
// it — so a session has to be able to read both back.
type Answer struct {
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
	// CostUSD is the bill the Decisions surface reports on its usage. System
	// One reports tokens but no price, so it stays zero there and a display
	// leaves it off rather than inventing a number.
	CostUSD float64
}

// Reporter is the optional half of Judge that a judge backed by a versioned
// remote service implements, so a caller can name what answered it without the
// transport leaking through the Judge interface. A judge that decides locally
// has no answer to report and simply does not implement it.
type Reporter interface {
	LastAnswer() Answer
}

// JudgeConfig is the opt-in classifier's settings. It is off by default:
// enabling it sends conversation history — source code, possibly secrets — to a
// third party, so it is never a shipped default.
type JudgeConfig struct {
	Enabled bool
	Model   string
	BaseURL string
	APIKey  string
	// Endpoint selects the wire surface: System One (the zero value, TypeSafe
	// direct) or the OpenRouter Decisions API that serves clef. The factory
	// derives it from the judge's provider.
	Endpoint jev.Endpoint
	// PruneThreshold is the prune probability a block must reach to be dropped.
	// A value outside (0,1] is unusable and falls back to DefaultPruneThreshold,
	// so a config that never mentions it cannot prune on any probability at all.
	PruneThreshold float64
	MaxBlocks      int
}

// ConfidenceFloor is how sure the judge must be before a verdict may change the
// conversation. It is deliberately a floor rather than a second ratio: the
// confidence is calibrated, so a low one means the distribution is flat and the
// safe answer is to keep everything.
const ConfidenceFloor = 0.6

// DefaultPruneThreshold is the prune probability a block must reach before it
// may be dropped, and the value an adapter falls back to when it is handed no
// usable one. Settings alias it, so the number cannot drift between the two
// layers.
//
// It was 0.85 until the calibration harness was pointed at the live model, which
// is the first time the number had ever been measured against anything. Over the
// labeled corpus, 0.85 pruned 17% of the blocks a careful operator would have
// dropped; 0.75 recovers 50% of them at the same zero false prunes, and lifts
// agreement with the labels from 65% to 76%. Recall is the cheaper thing to
// recover here — a missed prune costs a little context, and the confidence floor
// still refuses every verdict the model is unsure of, so the asymmetry that keeps
// a false prune impossible is untouched. See calibration_test.go and
// testdata/judge_labels.json; re-run them before moving this again.
const DefaultPruneThreshold = 0.75

const (
	optionKeep   = "keep"
	optionPrune  = "prune"
	optionLedger = "ledger"
)

// decide turns one block's answer into a verdict. The rule is asymmetric on
// purpose: a prune needs both a probability over the threshold and a confidence
// over the floor, while anything ambiguous, missing or empty is kept. The
// failure mode of a bad prune is a conversation that lost a fact; the failure
// mode of a bad keep is a little more context.
//
// The threshold must itself be usable, which is the second half of that
// asymmetry: a threshold of zero would be cleared by any prune probability at
// all, so an unusable one prunes nothing rather than everything. A caller who
// means "no pruning" gets a conversation instead of a deletion.
func decide(probabilities map[string]float64, choice string, confidence, threshold float64) Verdict {
	pruneProb, hasPrune := probabilities[optionPrune]
	verdict := Verdict{Decision: Keep, Choice: choice, PruneProb: pruneProb, Confidence: confidence}
	if confidence < ConfidenceFloor {
		return verdict
	}
	if hasPrune && threshold > 0 && pruneProb >= threshold {
		verdict.Decision = Prune
		return verdict
	}
	if choice == optionLedger {
		verdict.Decision = Ledger
	}
	return verdict
}

// keepAll is the safe answer: everything stays where it is.
func keepAll(n int) []Verdict {
	out := make([]Verdict, n)
	for i := range out {
		out[i] = Verdict{Decision: Keep}
	}
	return out
}
