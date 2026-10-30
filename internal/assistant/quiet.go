package assistant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

type quietFile struct {
	Until time.Time `json:"until"`
}

// InQuiet reports whether now falls in the quiet window or before the
// /quiet-until timestamp stored in quiet.json.
func InQuiet(now time.Time, a settings.Assistant, stateDir string) bool {
	if quietUntil(stateDir).After(now) {
		return true
	}
	a = a.WithDefaults()
	loc, err := time.LoadLocation(a.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	cur := local.Hour()*60 + local.Minute()
	start, ok1 := parseClock(a.QuietStart)
	end, ok2 := parseClock(a.QuietEnd)
	if !ok1 || !ok2 || start == end {
		return false
	}
	if start < end {
		return cur >= start && cur < end
	}
	return cur >= start || cur < end
}

func quietUntil(stateDir string) time.Time {
	data, err := os.ReadFile(filepath.Join(stateDir, "quiet.json"))
	if err != nil {
		return time.Time{}
	}
	var q quietFile
	if json.Unmarshal(data, &q) != nil {
		return time.Time{}
	}
	return q.Until
}

func parseClock(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// SetQuiet silences interrupts until the given time by writing quiet.json.
func SetQuiet(stateDir string, until time.Time) error {
	data, err := json.Marshal(quietFile{Until: until})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(stateDir, "quiet.json"), data)
}
