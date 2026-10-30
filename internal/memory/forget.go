package memory

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrEmptyQuery is returned by Forget for a blank query.
var ErrEmptyQuery = errors.New("memory: empty forget query")

// Forget removes every line containing query (case-insensitive) from USER.md,
// MEMORY.md and all notes, after backing each touched file up to
// backups/<name>.<timestamp>. It returns the number of lines removed.
func (s *Store) Forget(query string) (int, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0, ErrEmptyQuery
	}
	notes, err := filepath.Glob(s.path("notes", "*.md"))
	if err != nil {
		return 0, err
	}
	paths := append([]string{s.path(UserFile), s.path(MemoryFile)}, notes...)
	stamp := s.clock().Format("20060102-150405.000")
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, p := range paths {
		n, err := s.forgetIn(p, q, stamp)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (s *Store) forgetIn(path, q, stamp string) (int, error) {
	data, err := s.readFile(path)
	if err != nil || data == "" {
		return 0, err
	}
	kept := []string{}
	removed := 0
	for _, line := range splitLines(data) {
		if strings.Contains(strings.ToLower(line), q) {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return 0, nil
	}
	if err := s.backup(filepath.Base(path), stamp, data); err != nil {
		return 0, err
	}
	return removed, s.writeAtomic(path, []byte(joinLines(kept)))
}
