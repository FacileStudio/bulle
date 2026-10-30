package assistant

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

// Entry is what the ledger remembers about one item key.
type Entry struct {
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	LastSurfaced time.Time `json:"last_surfaced,omitempty"`
	Fingerprint  string    `json:"fingerprint,omitempty"`
	SnoozedUntil time.Time `json:"snoozed_until,omitempty"`
	Noise        bool      `json:"noise,omitempty"`
	LastStatus   string    `json:"last_status,omitempty"`
	Phase        string    `json:"phase,omitempty"`
}

// Ledger records what the assistant already told the user so the same
// unchanged item is never surfaced twice. It is safe for concurrent use.
type Ledger struct {
	mu      sync.Mutex
	path    string
	entries map[string]*Entry
}

// DefaultLedgerPath is ledger.json inside settings.StateDir.
func DefaultLedgerPath() (string, error) {
	dir, err := settings.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ledger.json"), nil
}

// NewLedger opens the ledger at path, or at DefaultLedgerPath when path is
// empty. A missing file starts empty. A corrupt file is copied to
// path+".corrupt" and the ledger starts empty.
func NewLedger(path string) (*Ledger, error) {
	if path == "" {
		var err error
		if path, err = DefaultLedgerPath(); err != nil {
			return nil, err
		}
	}
	l := &Ledger{path: path, entries: map[string]*Entry{}}
	return l, l.load()
}

func (l *Ledger) load() error {
	data, err := os.ReadFile(l.path)
	if errors.Is(err, os.ErrNotExist) {
		l.entries = map[string]*Entry{}
		return nil
	}
	if err != nil {
		return err
	}
	if json.Unmarshal(data, &l.entries) != nil || l.entries == nil {
		l.entries = map[string]*Entry{}
		return os.WriteFile(l.path+".corrupt", data, 0o600)
	}
	return nil
}

func (l *Ledger) lockPath() string {
	return filepath.Join(filepath.Dir(l.path), "ledger.lock")
}

func (l *Ledger) update(fn func()) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return withLock(l.lockPath(), true, func() error {
		if err := l.load(); err != nil {
			return err
		}
		fn()
		return l.save()
	})
}

func (l *Ledger) refresh() {
	_ = withLock(l.lockPath(), false, l.load)
}

func (l *Ledger) save() error {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l.entries, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ledger-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), l.path)
}
