// Package memory is bulle's bounded, file-backed assistant memory: a small
// profile, a small long-term file, add-only daily notes and backups.
package memory

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

const (
	// UserFile is the profile file name.
	UserFile = "USER.md"
	// MemoryFile is the long-term memory file name.
	MemoryFile = "MEMORY.md"
	// UserCap is the character cap of USER.md.
	UserCap = 1500
	// MemoryCap is the character cap of MEMORY.md.
	MemoryCap = 2500
	// MaxFact is the longest fact Remember accepts.
	MaxFact = 500
)

var (
	// ErrEmptyFact is returned for a blank fact.
	ErrEmptyFact = errors.New("memory: empty fact")
	// ErrFactTooLong is returned for a fact over MaxFact characters.
	ErrFactTooLong = errors.New("memory: fact too long")
	// ErrOverCap is returned when a write would exceed a file's cap.
	ErrOverCap = errors.New("memory: write would exceed the file cap")
	// ErrVerify is returned when the read-back after a write does not match.
	ErrVerify = errors.New("memory: read-back verification failed")
)

// Store owns one memory directory.
type Store struct {
	dir   string
	mu    sync.Mutex
	clock func() time.Time
}

// New returns a Store rooted at dir.
func New(dir string) *Store {
	return &Store{dir: dir, clock: time.Now}
}

// Default returns a Store rooted at memory/ inside settings.DataDir.
func Default() (*Store, error) {
	dir, err := settings.DataDir()
	if err != nil {
		return nil, err
	}
	return New(filepath.Join(dir, "memory")), nil
}

func (s *Store) path(parts ...string) string {
	return filepath.Join(append([]string{s.dir}, parts...)...)
}

func (s *Store) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

func (s *Store) writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (s *Store) backup(name, suffix, data string) error {
	return s.writeAtomic(s.path("backups", name+"."+suffix), []byte(data))
}
