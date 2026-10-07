package jev

import "encoding/json"

// decisionsError is the OpenRouter Decisions error envelope, {error:{code, message}}.
// System One reports its errors as a plain body behind the status code, so the
// two envelopes are read with two branches and a caller gets the same typed
// errors whichever surface the client is pointed at.
type decisionsError struct {
	Error struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// errorFor maps one non-2xx answer to its typed error. Decisions' envelope is
// unwrapped to its message, so a 401 reads "Missing Authentication header"
// rather than the whole JSON body; a body that is not an envelope, such as a
// proxy's HTML error page, falls through to the raw text. System One always
// takes the raw text, byte-identical to how this client read errors before the
// Decisions surface existed.
func errorFor(surface Endpoint, status int, body string) error {
	if surface == EndpointDecisions {
		var env decisionsError
		if err := json.Unmarshal([]byte(body), &env); err == nil && env.Error.Message != "" {
			return statusError(status, env.Error.Message)
		}
	}
	return statusError(status, body)
}
