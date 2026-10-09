package compaction

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// answerServer is a stub systemone endpoint: it records the questions it was
// asked and answers with the scripted body.
func answerServer(t *testing.T, body string, asked *map[string]json.RawMessage, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var request struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if *asked == nil {
			*asked = make(map[string]json.RawMessage)
		}
		maps.Copy(*asked, request.Questions)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// N blocks become N questions in one batched call, and each answer maps back to
// the block it was asked about.
func TestJevJudgeAsksOneBatchedQuestionPerBlock(t *testing.T) {
	var asked map[string]json.RawMessage
	var requests atomic.Int32
	server := answerServer(t, `{"answers":{
		"block-1":{"choice":"prune","confidence":0.95,"probabilities":{"prune":0.9,"keep":0.1}},
		"block-3":{"choice":"ledger","confidence":0.9,"probabilities":{"ledger":0.8}}}}`, &asked, &requests)

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, APIKey: "k", PruneThreshold: 0.85, MaxBlocks: 64})
	conv := judgeSample()

	verdicts, err := judge.Classify(t.Context(), "the task", Blocks(conv, judgePlan(conv)))
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	if got := requests.Load(); got != 1 {
		t.Errorf("requests = %d, want the questions batched into one", got)
	}
	if len(asked) != 2 {
		t.Errorf("questions = %d, want one per block", len(asked))
	}
	if len(verdicts) != 2 || verdicts[0].Decision != Prune || verdicts[1].Decision != Ledger {
		t.Errorf("verdicts = %+v, want the answers mapped back to their blocks", verdicts)
	}
}

// A call that fails degrades to all-keep and reports the error, so a 429 costs
// a fallback rather than a prune nobody can undo.
func TestJevJudgeDegradesOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, PruneThreshold: 0.85})
	blocks := Blocks(judgeSample(), judgePlan(judgeSample()))

	verdicts, err := judge.Classify(t.Context(), "the task", blocks)
	if err == nil {
		t.Fatal("err = nil, want the rate limit reported")
	}
	for i, verdict := range verdicts {
		if verdict.Decision != Keep {
			t.Errorf("verdict %d = %v after a failed call, want keep", i, verdict.Decision)
		}
	}
}

// When blocks exceed max_blocks_per_call, they are chunked and asked about
// in multiple requests to avoid exceeding the context limit.
func TestJevJudgeChunksWhatItCannotBatch(t *testing.T) {
	var asked map[string]json.RawMessage
	var requests atomic.Int32
	server := answerServer(t, `{"answers":{"block-3":{"choice":"prune","confidence":0.95,"probabilities":{"prune":0.9}}}}`, &asked, &requests)

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, MaxBlocks: 1, PruneThreshold: 0.85})
	blocks := Blocks(judgeSample(), judgePlan(judgeSample()))

	_, err := judge.Classify(t.Context(), "the task", blocks)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("requests = %d, want two requests for two chunks", got)
	}
	if len(asked) != 2 {
		t.Errorf("questions = %d, want all blocks asked", len(asked))
	}
}

// A judge built without a prune threshold falls back to the shipped one rather
// than to "prune on any probability at all": a half-filled config must not
// invert the one destructive verdict. The answer here is a 0.20 prune
// probability, which clears a threshold of zero and nothing else.
func TestJevJudgeFallsBackToTheDefaultThreshold(t *testing.T) {
	var asked map[string]json.RawMessage
	var requests atomic.Int32
	server := answerServer(t, `{"answers":{"block-1":{"choice":"keep","confidence":0.95,"probabilities":{"prune":0.20,"keep":0.80}}}}`, &asked, &requests)

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, MaxBlocks: 64})
	conv := judgeSample()

	verdicts, err := judge.Classify(t.Context(), "the task", Blocks(conv, judgePlan(conv)))
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if verdicts[0].Decision != Keep {
		t.Errorf("verdict = %v, want keep — a 0.20 prune probability must not clear the shipped %v threshold",
			verdicts[0].Decision, DefaultPruneThreshold)
	}
}

// A disabled judge builds nothing, so an off setting cannot accidentally reach
// the network.
func TestNewJevJudgeIsNilWhenDisabled(t *testing.T) {
	if judge := NewJevJudge(JudgeConfig{}); judge != nil {
		t.Errorf("judge = %v, want nil while the setting is off", judge)
	}
}

// The count cap is not the only bound on a batch: a block carries a whole tool
// result, so the request is capped in bytes too. If blocks exceed the budget,
// they are chunked and asked in multiple requests.
func TestJevJudgeChunksWhatItCannotFitInTheStateBudget(t *testing.T) {
	var asked map[string]json.RawMessage
	var requests atomic.Int32
	server := answerServer(t, `{"answers":{}}`, &asked, &requests)

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, MaxBlocks: 64, PruneThreshold: 0.85})
	blocks := []Block{
		{Key: "old", Text: strings.Repeat("x", defaultMaxState/2)},
		{Key: "mid", Text: strings.Repeat("x", defaultMaxState/2)},
		{Key: "new", Text: "a small recent turn"},
	}

	_, err := judge.Classify(t.Context(), "the task", blocks)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("requests = %d, want the blocks chunked into two requests", got)
	}
	if len(asked) != 3 {
		t.Errorf("questions = %d, want all blocks asked", len(asked))
	}
}

// A single block heavier than the whole budget is still asked about. Folding it
// would hand the summarizer a turn the judge never saw while the call asked
// nothing at all — the one way a byte cap could silently disable the judge.
func TestJevJudgeAlwaysAsksAboutTheNewestBlock(t *testing.T) {
	var asked map[string]json.RawMessage
	var requests atomic.Int32
	server := answerServer(t, `{"answers":{}}`, &asked, &requests)

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, PruneThreshold: 0.85})
	blocks := []Block{{Key: "huge", Text: strings.Repeat("x", defaultMaxState*2)}}

	verdicts, err := judge.Classify(t.Context(), "the task", blocks)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(asked) != 1 {
		t.Errorf("questions = %d, want the newest block asked about whatever it weighs", len(asked))
	}
	if verdicts[0].Decision != Keep {
		t.Errorf("verdict = %v, want the answer for the one block that was asked", verdicts[0].Decision)
	}
}

// The versioned id and bill of the model that answered are kept. The configured
// model defaults to the vendor's drifting alias and the thresholds are tuned
// against one build behind it, so the id has to be readable before it can be
// pinned.
func TestJevJudgeReportsTheModelThatAnswered(t *testing.T) {
	var asked map[string]json.RawMessage
	var requests atomic.Int32
	server := answerServer(t, `{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1200,"output_tokens":0}}`, &asked, &requests)

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, PruneThreshold: 0.85})
	conv := judgeSample()

	if _, err := judge.Classify(t.Context(), "the task", Blocks(conv, judgePlan(conv))); err != nil {
		t.Fatalf("Classify: %v", err)
	}

	reporter, ok := judge.(Reporter)
	if !ok {
		t.Fatal("the judge does not report what answered it, so nothing can pin the model")
	}
	if answer := reporter.LastAnswer(); answer.Model != "jev-1.13.0" || answer.InputTokens != 1200 {
		t.Errorf("LastAnswer = %+v, want the version that answered and what it billed", answer)
	}
}

// A failed call leaves the last answer standing rather than blanking it: the
// version a reader is about to pin should not vanish because one pass hit a 429.
func TestJevJudgeKeepsTheLastAnswerAcrossAFailure(t *testing.T) {
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		if _, err := w.Write([]byte(`{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1200}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	judge := NewJevJudge(JudgeConfig{Enabled: true, BaseURL: server.URL, PruneThreshold: 0.85})
	conv := judgeSample()
	blocks := Blocks(conv, judgePlan(conv))
	if _, err := judge.Classify(t.Context(), "the task", blocks); err != nil {
		t.Fatalf("first Classify: %v", err)
	}

	fail = true
	if _, err := judge.Classify(t.Context(), "the task", blocks); err == nil {
		t.Fatal("second Classify: nil error, want the rate limit reported")
	}

	if answer := judge.(Reporter).LastAnswer(); answer.Model != "jev-1.13.0" {
		t.Errorf("LastAnswer = %+v after a failed pass, want the previous version kept", answer)
	}
}
