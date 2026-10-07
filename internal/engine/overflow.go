package engine

import "strings"

var tooLong = []string{
	"prompt is too long",
	"maximum context length",
	"context length",
	"context_length",
	"too many tokens",
	"input token count",
	"exceeds the maximum",
	"exceeding the maximum",
}

// DetectOverflow reports whether err is a context-length rejection.
func DetectOverflow(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, phrase := range tooLong {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}
