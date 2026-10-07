package compaction

import (
	"context"
	"sync"

	"github.com/FacileStudio/bulle/internal/jev"
)

// defaultMaxState bounds one classification request in bytes.
// max_blocks_per_call bounds how many blocks are asked about, but a block
// carries a whole tool result, so a handful of large ones is a request no
// decision model should be asked to read — and every byte of it is billed. Both
// caps are spent from the recent end, where a prune is most useful.
const defaultMaxState = 256 * 1024

// jevJudge is the decision-model adapter: one state, one choice question per
// block, one batched call, over whichever wire surface the config chose —
// System One or the OpenRouter Decisions API. It is the only place in this
// package that knows a network exists.
type jevJudge struct {
	client    *jev.Client
	threshold float64
	maxBlocks int

	// mu guards the last call's answer. Classify runs on the pass goroutine while
	// LastAnswer is read from the update loop, so the two never share these
	// fields unguarded.
	mu   sync.Mutex
	last Answer
}

// NewJevJudge builds the opt-in classifier, or nil when the judge is off — a
// small, tool-free surface the TUI can hold and test without a network. The
// config's endpoint picks the wire surface; a zero value speaks System One.
func NewJevJudge(cfg JudgeConfig) Judge {
	if !cfg.Enabled {
		return nil
	}
	return &jevJudge{
		client:    jev.New(jev.Config{Endpoint: cfg.Endpoint, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model}),
		threshold: pruneThreshold(cfg.PruneThreshold),
		maxBlocks: cfg.MaxBlocks,
	}
}

// pruneThreshold fills in a threshold an adapter was handed none of. A config
// that never mentions the key carries a zero, and a zero would be cleared by any
// prune probability at all — so the value a half-filled config falls back to is
// the shipped default, not "prune whenever the judge is not sure".
func pruneThreshold(threshold float64) float64 {
	if threshold <= 0 || threshold > 1 {
		return DefaultPruneThreshold
	}
	return threshold
}

// Classify asks every block's question in one call and maps the answers back.
// Blocks past either batch cap are not asked about and are folded instead: the
// summarizer can compress them, where an unclassified prune could not be taken
// back. A failed call returns all-keep verdicts and the error, so a caller that
// ignores the error still prunes nothing.
func (j *jevJudge) Classify(ctx context.Context, goal string, blocks []Block) ([]Verdict, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	verdicts := make([]Verdict, len(blocks))
	first := j.overflow(blocks, verdicts)
	asked := blocks[first:]

	response, err := j.client.Evaluate(ctx, state(goal, asked), questions(asked))
	if err != nil {
		return keepAll(len(blocks)), err
	}
	j.record(response)
	for i, block := range asked {
		answer := response.Answers[block.Key]
		verdicts[first+i] = decide(answer.Probabilities, answer.Choice, answer.Confidence, j.threshold)
	}
	return verdicts, nil
}

// overflow marks the oldest blocks that do not fit the batch as ledger material
// and returns the index the call may start at. It is the answer to "too much
// history for one call": classify the recent end where a prune is most useful,
// and let the summarizer compress the rest.
//
// Two caps bound a batch — the count configured by max_blocks_per_call, and
// defaultMaxState bytes of block text — and the newest block is admitted
// whatever it weighs, so a batch is never empty and a single oversized block
// cannot starve the call.
func (j *jevJudge) overflow(blocks []Block, verdicts []Verdict) int {
	first := len(blocks)
	budget := defaultMaxState
	for first > 0 {
		next := first - 1
		overCount := j.maxBlocks > 0 && len(blocks)-next > j.maxBlocks
		overBudget := first < len(blocks) && len(blocks[next].Text) > budget
		if overCount || overBudget {
			break
		}
		budget -= len(blocks[next].Text)
		first = next
	}
	for i := range first {
		verdicts[i] = Verdict{Decision: Ledger}
	}
	return first
}

// record keeps what the last call answered and billed. It is called only on a
// successful call, so a failed pass never blanks the version a reader is looking
// at.
func (j *jevJudge) record(response jev.Response) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.last = Answer{
		Model:        response.Model,
		Provider:     response.Provider,
		InputTokens:  response.Usage.InputTokens,
		OutputTokens: response.Usage.OutputTokens,
		CostUSD:      response.Usage.Cost,
	}
}

// LastAnswer is the model version and the bill of the most recent call, zero
// before one has happened. TypeSafe answers with a versioned id behind a
// drifting `jev-latest` alias and says to log it and pin it once the thresholds
// have been tuned against it, so a session has to be able to read it back — this
// is what /status shows.
func (j *jevJudge) LastAnswer() Answer {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.last
}
