package jev

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const decisionsAnswer = `{
	"id":"gen-dec-1789738314-X5e5eKGQdvR9rblyX250",
	"model":"cloudflare/clef",
	"provider":"Cloudflare",
	"answers":{
		"a":{"type":"choice","choice":"prune","confidence":0.9,"probabilities":{"prune":0.9,"keep":0.1}},
		"b":{"type":"noul","noul":0.96}},
	"usage":{"cost":0.000019992,"input_tokens":476,"output_tokens":70}}`

// decisionsRecord is what a stub Decisions endpoint saw: the requests, the auth
// header, the path, and the state as raw JSON so a string state and an object
// state can both be asserted through one field.
type decisionsRecord struct {
	requests  atomic.Int32
	auth      string
	path      string
	state     json.RawMessage
	model     string
	questions map[string]Question
}

// decisionsServer answers every request with body and records what it was asked,
// so a test can assert on both sides of the wire.
func decisionsServer(t *testing.T, body string, rec *decisionsRecord) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.requests.Add(1)
		rec.auth, rec.path = r.Header.Get("Authorization"), r.URL.Path
		var sent struct {
			Model     string              `json:"model"`
			State     json.RawMessage     `json:"state"`
			Questions map[string]Question `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("decode request: %v", err)
		}
		rec.model, rec.state, rec.questions = sent.Model, sent.State, sent.Questions
		writeBody(t, w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func newDecisionsClient(url string) *Client {
	return New(Config{BaseURL: url, APIKey: "test-key", Model: "cloudflare/clef", Endpoint: EndpointDecisions, Attempts: 3, Backoff: time.Millisecond})
}

func errorBody(t *testing.T, w http.ResponseWriter, status int, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeBody(t, w, body)
}

// A Decisions client posts to /api/alpha/decisions on the caller's base URL with
// the same three-field body a System One client sends, and an object state as a
// JSON object rather than a text blob.
func TestDecisionsSendsToTheDecisionsPath(t *testing.T) {
	var rec decisionsRecord
	server := decisionsServer(t, decisionsAnswer, &rec)

	response, err := newDecisionsClient(server.URL).Evaluate(t.Context(), map[string]string{"goal": "ship"}, twoQuestions())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if rec.path != decisionsPath {
		t.Errorf("path = %q, want %q", rec.path, decisionsPath)
	}
	if rec.auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want the bearer key", rec.auth)
	}
	if rec.model != "cloudflare/clef" {
		t.Errorf("model = %q, want the configured model", rec.model)
	}
	var state struct {
		Goal string `json:"goal"`
	}
	if err := json.Unmarshal(rec.state, &state); err != nil {
		t.Fatalf("state = %s, want a JSON object", rec.state)
	}
	if state.Goal != "ship" {
		t.Errorf("state.goal = %q, want the object state passed through", state.Goal)
	}
	if len(rec.questions) != 2 || rec.questions["a"].Type != "choice" {
		t.Errorf("questions = %v, want both choice questions", rec.questions)
	}
	if response.Answers["a"].Choice != "prune" || response.Answers["a"].Confidence != 0.9 {
		t.Errorf("answer a = %+v, want the choice and its confidence", response.Answers["a"])
	}
	if response.Answers["b"].Noul != 0.96 {
		t.Errorf("answer b = %+v, want the noul answer", response.Answers["b"])
	}
}

// A string state still encodes as a plain string on the Decisions surface, so a
// caller that builds state as text today needs no change.
func TestDecisionsAcceptsAStringState(t *testing.T) {
	var rec decisionsRecord
	server := decisionsServer(t, decisionsAnswer, &rec)

	if _, err := newDecisionsClient(server.URL).Evaluate(t.Context(), "state text", twoQuestions()); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	var state string
	if err := json.Unmarshal(rec.state, &state); err != nil {
		t.Fatalf("state = %s, want a JSON string", rec.state)
	}
	if state != "state text" {
		t.Errorf("state = %q, want the text passed through", state)
	}
}

// The Decisions surface reports cost, id and provider; TypeSafe direct never
// does, and all three read into the shared types.
func TestDecisionsParsesCost(t *testing.T) {
	server := decisionsServer(t, decisionsAnswer, &decisionsRecord{})

	response, err := newDecisionsClient(server.URL).Evaluate(t.Context(), "state", twoQuestions())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if response.Usage.Cost != 0.000019992 {
		t.Errorf("usage.cost = %v, want the billed cost", response.Usage.Cost)
	}
	if response.Usage.InputTokens != 476 || response.Usage.OutputTokens != 70 {
		t.Errorf("usage = %+v, want the billed tokens", response.Usage)
	}
	if response.ID == "" || response.Provider != "Cloudflare" {
		t.Errorf("id/provider = %q/%q, want the Decisions identifiers", response.ID, response.Provider)
	}
}

// A System One body decodes as before: no cost field means a zero cost, and no
// id or provider where the surface never sends either.
func TestSystemOneParsesWithoutACost(t *testing.T) {
	server := recordingServer(t, batchAnswer, &recorded{})

	response, err := newTestClient(server.URL).Evaluate(t.Context(), "state", twoQuestions())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if response.Usage.Cost != 0 {
		t.Errorf("usage.cost = %v, want zero for a System One response", response.Usage.Cost)
	}
	if response.Usage.InputTokens != 120 || response.ID != "" || response.Provider != "" {
		t.Errorf("response = %+v, want the old shape unchanged", response)
	}
}
