package memory

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const delimiter = "\n§\n"

// Snapshot renders USER.md and MEMORY.md as one prompt block with usage
// counts. Empty files render nothing. The caller freezes it per session.
func (s *Store) Snapshot(_ time.Time) string {
	blocks := []string{}
	for _, f := range []struct{ file, label string }{{UserFile, "USER PROFILE"}, {MemoryFile, "MEMORY"}} {
		text, _ := s.readFile(s.path(f.file))
		limit, _ := capOf(f.file)
		if b := renderBlock(f.label, strings.TrimSpace(text), limit); b != "" {
			blocks = append(blocks, b)
		}
	}
	return strings.Join(blocks, delimiter)
}

func renderBlock(label, text string, limit int) string {
	if text == "" {
		return ""
	}
	n := utf8.RuneCountInString(text)
	return fmt.Sprintf("%s [%d%% %s/%s chars]\n%s", label, n*100/limit, thousands(n), thousands(limit), text)
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	return thousands(n/1000) + "," + s[len(s)-3:]
}
