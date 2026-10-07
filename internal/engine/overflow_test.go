package engine

import (
	"errors"
	"testing"
)

func TestDetectOverflowRecognisesWording(t *testing.T) {
	refusals := []string{
		"prompt is too long: 213000 tokens > 200000 maximum",
		"This model's maximum context length is 128000 tokens. However, your messages resulted in 145000 tokens.",
		"maximum context length is 131072 tokens, you requested 140000",
		"invalid_request_error: context_length_exceeded",
		"The input token count (1050000) exceeds the maximum number of tokens allowed (1000000)",
		"Prompt contains 40000 tokens, exceeding the maximum context length",
		"too many tokens in the request",
	}

	for _, message := range refusals {
		if !DetectOverflow(errors.New(message)) {
			t.Errorf("DetectOverflow(%q) = false, want true", message)
		}
	}
}

func TestDetectOverflowLeavesOtherFailures(t *testing.T) {
	others := []string{
		"429 Too Many Requests: rate limit exceeded",
		"401 invalid api key",
		"context deadline exceeded",
		"the tool returned no output",
	}

	for _, message := range others {
		if DetectOverflow(errors.New(message)) {
			t.Errorf("DetectOverflow(%q) = true, want false", message)
		}
	}
	if DetectOverflow(nil) {
		t.Error("DetectOverflow(nil) = true, want false")
	}
}
