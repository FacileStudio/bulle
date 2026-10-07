package jev

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Every transient status is retried on the Decisions surface too, envelope or
// plain body alike. The set is the one errors.go already retries, plus 524,
// which the Decisions spec calls out as an edge timeout.
func TestDecisionsRetriesTransientFailures(t *testing.T) {
	for _, status := range []int{
		http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout, 524, 529,
	} {
		t.Run(fmt.Sprintf("%d", status), func(t *testing.T) {
			decisionsRetriesOneStatus(t, status)
		})
	}
}

func decisionsRetriesOneStatus(t *testing.T, status int) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			errorBody(t, w, status, fmt.Sprintf(`{"error":{"code":%d,"message":"retry me"}}`, status))
			return
		}
		writeBody(t, w, decisionsAnswer)
	}))
	defer server.Close()

	response, err := newDecisionsClient(server.URL).Evaluate(t.Context(), "state", map[string]Question{"a": {Type: "choice"}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("requests = %d, want a retry after the %d", got, status)
	}
	if response.Answers["a"].Choice != "prune" {
		t.Errorf("answer = %+v, want the retried answer", response.Answers["a"])
	}
}

// A 400 and a 401 are the endpoint speaking, not the network failing, so they
// are never retried on either surface. 500 is a real reply too — the client
// retries only the gateway and overload codes it already did.
func TestDecisionsNeverRetriesAFinalFailure(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError} {
		t.Run(fmt.Sprintf("%d", status), func(t *testing.T) {
			decisionsNeverRetriesOneStatus(t, status)
		})
	}
}

func decisionsNeverRetriesOneStatus(t *testing.T, status int) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		errorBody(t, w, status, fmt.Sprintf(`{"error":{"code":%d,"message":"nope"}}`, status))
	}))
	defer server.Close()

	_, err := newDecisionsClient(server.URL).Evaluate(t.Context(), "state", map[string]Question{"a": {Type: "choice"}})
	if err == nil {
		t.Fatal("err = nil, want the endpoint's error")
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("requests = %d, want exactly one attempt", got)
	}
}

// The retry policy is one list for both surfaces: 429, the gateway and overload
// codes, and 524 are retried; everything else a caller would read as a verdict
// is final, including 500.
func TestTransientCoversTheSpecStatuses(t *testing.T) {
	retried := []int{http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout, 524, 529}
	final := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired,
		http.StatusForbidden, http.StatusNotFound, http.StatusRequestEntityTooLarge, http.StatusInternalServerError}
	for _, status := range retried {
		if !transient(status) {
			t.Errorf("transient(%d) = false, want the status retried", status)
		}
	}
	for _, status := range final {
		if transient(status) {
			t.Errorf("transient(%d) = true, want the status to be final", status)
		}
	}
}
