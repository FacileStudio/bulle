package jev

import (
	"errors"
	"fmt"
	"net/http"
)

// UnauthorizedError is a 401: the key is missing, wrong, or not enabled for
// System One. It is not retryable, and it is the one failure an operator must
// see named rather than folded into a generic HTTP error.
type UnauthorizedError struct{ Detail string }

func (e *UnauthorizedError) Error() string {
	return "typesafe: unauthorized (401): " + e.Detail
}

// InvalidRequestError is a 422: the request itself is malformed — a bad option
// count, an unknown primitive. Retrying it unchanged cannot help.
type InvalidRequestError struct{ Detail string }

func (e *InvalidRequestError) Error() string {
	return "typesafe: invalid request (422): " + e.Detail
}

// HTTPError is any other non-2xx answer. 429 and 529 are retried by the client
// before this ever surfaces; a 5xx that survives every attempt comes back here.
type HTTPError struct {
	Status int
	Detail string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("typesafe: http %d: %s", e.Status, e.Detail)
}

// statusError maps a status to its typed error, so a caller can tell an auth
// problem from a malformed request without parsing a string.
func statusError(status int, detail string) error {
	switch status {
	case http.StatusUnauthorized:
		return &UnauthorizedError{Detail: detail}
	case http.StatusUnprocessableEntity:
		return &InvalidRequestError{Detail: detail}
	default:
		return &HTTPError{Status: status, Detail: detail}
	}
}

// retryable reports whether an error is the transient kind the client backs off
// and retries. Two kinds are: a rate limit or an overloaded backend, and a
// transport that never produced an answer at all — a dropped connection or a
// per-attempt timeout, which is exactly the blip a second attempt clears. Two are
// not: a typed 401 or 422 cannot succeed unchanged, and a 4xx or 5xx with a body
// is the endpoint speaking, not the network failing.
func retryable(err error) bool {
	if _, ok := errors.AsType[*UnauthorizedError](err); ok {
		return false
	}
	if _, ok := errors.AsType[*InvalidRequestError](err); ok {
		return false
	}
	if httpErr, ok := errors.AsType[*HTTPError](err); ok {
		return transient(httpErr.Status)
	}
	return true
}

// transient reports whether a status is one a second attempt can clear.
//
// A gateway's 502, 503, 504 or 524 is a transport failure wearing a status
// code: the request never reached the endpoint, so nothing was asked and
// nothing was decided, and treating it as the endpoint speaking would spend the
// whole attempt budget on a pass that has to fall back anyway. 524 is the edge
// timeout the Decisions surface calls out. 429 and 529 are the surfaces' own
// retryable answers; every other status is a real reply.
func transient(status int) bool {
	switch status {
	case http.StatusTooManyRequests, 524, 529,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
