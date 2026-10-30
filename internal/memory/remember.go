package memory

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Remember appends "- [HH:MM] fact" to today's note and returns the exact
// line written. It succeeds only after the file is written and read back. A
// fact already kept, in USER.md, MEMORY.md or today's note, is not written
// again and its existing line is returned.
func (s *Store) Remember(fact string, now time.Time) (string, error) {
	fact, err := cleanFact(fact)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.known(fact); ok {
		return existing, nil
	}
	path := s.path("notes", now.Format("2006-01-02")+".md")
	old, err := s.readFile(path)
	if err != nil {
		return "", err
	}
	if existing, ok := inNote(old, fact); ok {
		return existing, nil
	}
	return s.appendNote(path, old, fmt.Sprintf("- [%s] %s", now.Format("15:04"), fact))
}

func cleanFact(fact string) (string, error) {
	fact = strings.Join(strings.Fields(fact), " ")
	if fact == "" {
		return "", ErrEmptyFact
	}
	if utf8.RuneCountInString(fact) > MaxFact {
		return "", ErrFactTooLong
	}
	return fact, checkFact(fact)
}

func (s *Store) appendNote(path, old, line string) (string, error) {
	if err := s.writeAtomic(path, []byte(withNewline(old)+line+"\n")); err != nil {
		return "", err
	}
	got, err := s.readFile(path)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(got, line+"\n") {
		return "", ErrVerify
	}
	return line, nil
}

// Append adds one line to USER.md or MEMORY.md, refusing a write that would
// push the file over its cap.
func (s *Store) Append(name, line string) error {
	limit, ok := capOf(name)
	if !ok {
		return fmt.Errorf("memory: %s is not a bounded file", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.readFile(s.path(name))
	if err != nil {
		return err
	}
	next := withNewline(old) + strings.TrimSpace(line) + "\n"
	if utf8.RuneCountInString(strings.TrimSpace(next)) > limit {
		return ErrOverCap
	}
	return s.writeAtomic(s.path(name), []byte(next))
}

func withNewline(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

func capOf(name string) (int, bool) {
	switch name {
	case UserFile:
		return UserCap, true
	case MemoryFile:
		return MemoryCap, true
	}
	return 0, false
}
