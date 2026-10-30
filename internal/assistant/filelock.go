package assistant

import (
	"errors"
	"path/filepath"
)

// ErrLocked means another assistant run holds the run lock.
var ErrLocked = errors.New("assistant: another run holds the lock")

func withLock(path string, exclusive bool, fn func() error) error {
	unlock, err := lockFile(path, exclusive, true)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// AcquireRun takes the non-blocking run lock in stateDir. It returns
// ErrLocked when another watch or digest is already running.
func AcquireRun(stateDir string) (func(), error) {
	return lockFile(filepath.Join(stateDir, "run.lock"), true, false)
}
