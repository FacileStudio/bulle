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
const defaultMaxState = 32 * 1024

// jevJudge is the decision-model adapter: one state, one choice question per
// block, batched in chunks over whichever wire surface the config chose —
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

// NewJevJudge builds the opt-in classifier, or nil when the judge is off.
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

func pruneThreshold(threshold float64) float64 {
	if threshold <= 0 || threshold > 1 {
		return DefaultPruneThreshold
	}
	return threshold
}

type blockChunk struct {
	offset int
	blocks []Block
}

// Classify asks every block's question in chunks and maps the answers back.
// By splitting the state into chunks, we avoid exceeding the model's max tokens.
// A failed call returns all-keep verdicts and the error, so a caller that
// ignores the error still prunes nothing.
func (j *jevJudge) Classify(ctx context.Context, goal string, blocks []Block) ([]Verdict, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	verdicts := make([]Verdict, len(blocks))
	chunks := j.chunkBlocks(blocks)

	for _, chunk := range chunks {
		response, err := j.client.Evaluate(ctx, state(goal, chunk.blocks), questions(chunk.blocks))
		if err != nil {
			return keepAll(len(blocks)), err
		}
		j.record(response)
		for i, block := range chunk.blocks {
			answer := response.Answers[block.Key]
			verdicts[chunk.offset+i] = decide(answer.Probabilities, answer.Choice, answer.Confidence, j.threshold)
		}
	}
	return verdicts, nil
}

// chunkBlocks splits the blocks into safe-sized batches to avoid exceeding
// the context limit of the decision model.
func (j *jevJudge) chunkBlocks(blocks []Block) []blockChunk {
	var chunks []blockChunk
	start := 0
	for start < len(blocks) {
		end := start + 1
		budget := defaultMaxState - len(blocks[start].Text)

		for end < len(blocks) {
			overCount := j.maxBlocks > 0 && (end-start+1) > j.maxBlocks
			overBudget := len(blocks[end].Text) > budget
			if overCount || overBudget {
				break
			}
			budget -= len(blocks[end].Text)
			end++
		}

		chunks = append(chunks, blockChunk{
			offset: start,
			blocks: blocks[start:end],
		})
		start = end
	}
	return chunks
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
