package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Restore puts the file saved as backups/<backup> back in place, for any
// backup Forget, Consolidate or Revert wrote. The current file is kept as
// backups/<file>.pre-restore.<timestamp> first.
func (s *Store) Restore(backup string) error {
	if backup != filepath.Base(backup) {
		return fmt.Errorf("memory: backup name %q must not contain a path", backup)
	}
	cut := strings.Index(backup, ".md.")
	if cut < 0 {
		return fmt.Errorf("memory: %q is not a backup name", backup)
	}
	name := backup[:cut+3]
	data, err := os.ReadFile(s.path("backups", backup))
	if err != nil {
		return fmt.Errorf("memory: no backup %s: %w", backup, err)
	}
	target := s.path(name)
	if name != UserFile && name != MemoryFile {
		target = s.path("notes", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readFile(target)
	if err != nil {
		return err
	}
	if current != "" {
		stamp := s.clock().Format("20060102-150405.000")
		if err := s.backup(name, "pre-restore."+stamp, current); err != nil {
			return err
		}
	}
	return s.writeAtomic(target, data)
}
