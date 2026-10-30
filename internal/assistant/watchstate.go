package assistant

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

const (
	maxSince   = 72 * time.Hour
	firstSince = 24 * time.Hour
	sinceSlack = 5 * time.Minute
)

type watchState struct {
	Last time.Time `json:"last"`
}

// LastWatch returns the time of the last complete watch run, zero if none.
func LastWatch(stateDir string) time.Time {
	data, err := os.ReadFile(filepath.Join(stateDir, "watch.json"))
	if err != nil {
		return time.Time{}
	}
	var s watchState
	if json.Unmarshal(data, &s) != nil {
		return time.Time{}
	}
	return s.Last
}

func saveLastWatch(stateDir string, at time.Time) error {
	data, err := json.Marshal(watchState{Last: at})
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(stateDir, "watch.json"), data)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
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
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func sinceArg(last, now time.Time) string {
	gap := firstSince
	if !last.IsZero() {
		gap = min(now.Sub(last)+sinceSlack, maxSince)
	}
	return fmt.Sprintf("%dm", int(math.Ceil(max(gap, time.Minute).Minutes())))
}
