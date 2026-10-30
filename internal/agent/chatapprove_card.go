package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	valueLimit = 80
	fieldLimit = 8
)

var pathLike = []string{"path", "file", "dir", "url", "root", "pattern"}

func describe(name string, fields map[string]json.RawMessage, input json.RawMessage) (string, bool) {
	if name == "run_command" {
		var in shellCommandInput
		if json.Unmarshal(input, &in) == nil && in.Command != "" {
			return "`" + cardText(in.Command) + "`", ReadOnly(in.Command)
		}
	}
	return fmt.Sprintf("l'outil `%s` avec %s", cardText(name), fieldsSummary(fields)), false
}

func cardText(s string) string {
	return strings.ReplaceAll(s, "`", "'")
}

func fieldsSummary(fields map[string]json.RawMessage) string {
	keys := orderedKeys(fields)
	elided := len(keys) > fieldLimit
	if elided {
		keys = keys[:fieldLimit]
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value, cut := elide(fieldValue(fields[key]), valueLimit)
		elided = elided || cut
		parts = append(parts, cardText(key)+"="+value)
	}
	text := strings.Join(parts, ", ")
	if elided {
		text += " (tronqué)"
	}
	return text
}

func orderedKeys(fields map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, pj := isPathLike(keys[i]), isPathLike(keys[j])
		if pi != pj {
			return pi
		}
		return keys[i] < keys[j]
	})
	return keys
}

func isPathLike(key string) bool {
	key = strings.ToLower(key)
	for _, hint := range pathLike {
		if strings.Contains(key, hint) {
			return true
		}
	}
	return false
}

func fieldValue(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return cardText(strings.Map(flatten, text))
	}
	return cardText(string(raw))
}

func flatten(r rune) rune {
	if r < ' ' || r == 0x7f {
		return ' '
	}
	return r
}

func elide(s string, limit int) (string, bool) {
	runes := []rune(s)
	if len(runes) <= limit {
		return s, false
	}
	return string(runes[:limit]) + "...", true
}
