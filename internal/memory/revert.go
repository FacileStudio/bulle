package memory

import (
	"fmt"
	"os"
	"time"
)

func (s *Store) keepBackup(date, data string) error {
	if _, err := os.Stat(s.path("backups", MemoryFile+"."+date)); err == nil {
		return nil
	}
	return s.backup(MemoryFile, date, data)
}

// Revert restores MEMORY.md byte for byte from backups/MEMORY.md.<date>,
// where date is YYYY-MM-DD.
func (s *Store) Revert(date string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("memory: revert date must be YYYY-MM-DD: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path("backups", MemoryFile+"."+date))
	if err != nil {
		return fmt.Errorf("memory: no backup for %s: %w", date, err)
	}
	return s.writeAtomic(s.path(MemoryFile), data)
}
