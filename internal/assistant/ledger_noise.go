package assistant

import (
	"path/filepath"
	"strings"
	"time"
)

// MarkNoise silences a mail key for good and drops its sender from future
// fetches. Tasks and events are never marked as noise.
func (l *Ledger) MarkNoise(key string) error {
	return l.MarkNoiseAt(key, time.Now())
}

// MarkNoiseAt is MarkNoise with an explicit clock.
func (l *Ledger) MarkNoiseAt(key string, now time.Time) error {
	if !strings.HasPrefix(key, string(SourceMail)+":") {
		return nil
	}
	err := l.update(func() {
		l.touch(key, now).Noise = true
	})
	if err != nil {
		return err
	}
	return DropSenderOf(filepath.Dir(l.path), key)
}
