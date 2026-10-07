package jev

import "testing"

// A Config that never names an endpoint keeps the System One path and the
// default host, byte-identical to a client built before the Decisions surface
// existed. A zero-value Client, which skips New entirely, resolves the same.
func TestZeroConfigKeepsTheSystemOnePath(t *testing.T) {
	var zeroValue Client
	if zeroValue.path() != endpoint {
		t.Errorf("zero-value path = %q, want %q", zeroValue.path(), endpoint)
	}
	if got := New(Config{}).path(); got != endpoint {
		t.Errorf("zero-Config path = %q, want %q", got, endpoint)
	}
	if got := New(Config{Endpoint: EndpointSystemOne}).path(); got != endpoint {
		t.Errorf("explicit System One path = %q, want %q", got, endpoint)
	}
	if got := New(Config{Endpoint: EndpointDecisions}).path(); got != decisionsPath {
		t.Errorf("Decisions path = %q, want %q", got, decisionsPath)
	}
	if got := New(Config{}).baseURL; got != DefaultBaseURL {
		t.Errorf("zero-Config base URL = %q, want %q", got, DefaultBaseURL)
	}
	if DefaultBaseURL != "https://api.typesafe.ai" {
		t.Errorf("DefaultBaseURL = %q, want the TypeSafe host unchanged", DefaultBaseURL)
	}
}

// Decisions defaults its base URL to OpenRouter and resolves /api prefixes
// cleanly without doubling them.
func TestDecisionsDefaultBaseURLAndURL(t *testing.T) {
	if got := New(Config{Endpoint: EndpointDecisions}).baseURL; got != "https://openrouter.ai" {
		t.Errorf("Decisions default baseURL = %q, want https://openrouter.ai", got)
	}
	if got := New(Config{Endpoint: EndpointDecisions, BaseURL: "https://openrouter.ai/api"}).url(); got != "https://openrouter.ai/api/alpha/decisions" {
		t.Errorf("Decisions url with /api suffix = %q, want https://openrouter.ai/api/alpha/decisions", got)
	}
	if got := New(Config{Endpoint: EndpointDecisions, BaseURL: "https://openrouter.ai"}).url(); got != "https://openrouter.ai/api/alpha/decisions" {
		t.Errorf("Decisions url = %q, want https://openrouter.ai/api/alpha/decisions", got)
	}
}

// The default is not just a resolved path: a Config that names no endpoint also
// posts to /v1/systemone on the wire, with the model string ignored for routing.
func TestZeroConfigPostsToSystemOne(t *testing.T) {
	var rec recorded
	server := recordingServer(t, batchAnswer, &rec)

	_, err := New(Config{BaseURL: server.URL, APIKey: "test-key", Model: "cloudflare/clef"}).
		Evaluate(t.Context(), "state", map[string]Question{"a": {Type: "choice"}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rec.path != endpoint {
		t.Errorf("path = %q, want %q", rec.path, endpoint)
	}
}
