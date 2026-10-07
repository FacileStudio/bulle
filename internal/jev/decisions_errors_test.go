package jev

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Decisions error envelope, {error:{code,message}}, is unwrapped to its
// message, so a 401 reads "Missing Authentication header" rather than the whole
// JSON body. A body that is not an envelope, such as a proxy's HTML error page,
// keeps the raw text.
func TestDecisionsErrorEnvelopeUnwraps(t *testing.T) {
	decisionsError := func(t *testing.T, status int, body string, detail string) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			errorBody(t, w, status, body)
		}))
		defer server.Close()

		_, err := New(Config{BaseURL: server.URL, APIKey: "test-key", Endpoint: EndpointDecisions, Attempts: 1}).
			Evaluate(t.Context(), "state", map[string]Question{"a": {Type: "choice"}})
		if err == nil {
			t.Fatal("err = nil, want a typed error")
		}
		if !strings.Contains(err.Error(), detail) {
			t.Errorf("err = %q, want the detail %q", err, detail)
		}
	}

	t.Run("unauthorized", func(t *testing.T) {
		decisionsError(t, http.StatusUnauthorized,
			`{"error":{"code":401,"message":"Missing Authentication header"}}`, "Missing Authentication header")
	})
	t.Run("bad request", func(t *testing.T) {
		decisionsError(t, http.StatusBadRequest,
			`{"error":{"code":400,"message":"Invalid request parameters"}}`, "Invalid request parameters")
	})
	t.Run("string error code", func(t *testing.T) {
		decisionsError(t, http.StatusUnauthorized,
			`{"error":{"code":"invalid_api_key","message":"Invalid API key provided"}}`, "Invalid API key provided")
	})
	t.Run("omitted error code", func(t *testing.T) {
		decisionsError(t, http.StatusTooManyRequests,
			`{"error":{"message":"Rate limit exceeded"}}`, "Rate limit exceeded")
	})
	t.Run("non-envelope", func(t *testing.T) {
		decisionsError(t, http.StatusNotFound, "nope\n", "nope")
	})
}

// System One reports its errors as a plain body behind the status code, and it
// always did — so a 401 still reads as a typed UnauthorizedError, byte for
// byte as before the Decisions surface existed.
func TestSystemOneErrorBodyIsUntouched(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		errorBody(t, w, http.StatusUnauthorized, "nope\n")
	}))
	defer server.Close()

	_, err := New(Config{BaseURL: server.URL, APIKey: "test-key", Endpoint: EndpointSystemOne, Attempts: 1}).
		Evaluate(t.Context(), "state", map[string]Question{"a": {Type: "choice"}})
	if err == nil {
		t.Fatal("err = nil, want an UnauthorizedError")
	}
	if _, ok := errors.AsType[*UnauthorizedError](err); !ok {
		t.Fatalf("err = %v, want an UnauthorizedError", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("err = %q, want the plain body detail", err)
	}
}
