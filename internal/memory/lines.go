package memory

import (
	"strings"
	"unicode/utf8"
)

func splitLines(text string) []string {
	out := []string{}
	for l := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func dropExact(lines, drops []string, limit int) (kept, dropped []string) {
	want := map[string]bool{}
	for _, d := range drops {
		want[d] = true
	}
	for _, l := range lines {
		if want[l] && len(dropped) < limit {
			dropped = append(dropped, l)
			continue
		}
		kept = append(kept, l)
	}
	return kept, dropped
}

func factKey(line string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- ")))
}

func addFacts(lines, facts []string) (added, merged []string) {
	seen := map[string]bool{}
	for _, l := range lines {
		seen[factKey(l)] = true
	}
	merged = append([]string{}, lines...)
	for _, f := range facts {
		f = strings.Join(strings.Fields(f), " ")
		if f == "" || utf8.RuneCountInString(f) > MaxFact || seen[factKey(f)] {
			continue
		}
		seen[factKey(f)] = true
		line := "- " + strings.TrimPrefix(f, "- ")
		added = append(added, line)
		merged = append(merged, line)
	}
	return added, merged
}
