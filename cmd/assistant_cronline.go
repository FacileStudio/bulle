package cmd

import (
	"regexp"
	"strings"
)

var safeCronWord = regexp.MustCompile(`^[A-Za-z0-9_./:@+=,-]+$`)

func cronQuote(s string) string {
	if !safeCronWord.MatchString(s) {
		s = "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return strings.ReplaceAll(s, "%", `\%`)
}
