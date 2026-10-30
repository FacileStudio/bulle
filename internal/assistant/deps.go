package assistant

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/memory"
	"github.com/FacileStudio/bulle/internal/settings"
)

const (
	DeliveryOK         = "ok"
	DeliveryFailed     = "failed"
	DeliverySuppressed = "suppressed"
)

// Deps is everything Watch and Digest touch outside the process, so tests
// replace the clock, the commands, the ledger, the chat socket and the model.
type Deps struct {
	Now      func() time.Time
	Run      Runner
	Ledger   *Ledger
	Send     chat.IPCSend
	Complete memory.Completer
	Settings settings.Assistant
	StateDir string
	LogDir   string
	Sleep    func(time.Duration)
	Log      io.Writer
}

func (d Deps) withDefaults() (Deps, error) {
	d.Settings = d.Settings.WithDefaults()
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Run == nil {
		d.Run = ExecRunner
	}
	if d.Sleep == nil {
		d.Sleep = time.Sleep
	}
	if d.Log == nil {
		d.Log = os.Stderr
	}
	if d.Send == nil {
		d.Send = DefaultSend
	}
	return d.withDirs()
}

func (d Deps) withDirs() (Deps, error) {
	state, err := settings.StateDir()
	if err != nil {
		return d, err
	}
	if d.StateDir == "" {
		d.StateDir = state
	}
	if d.LogDir == "" {
		d.LogDir = filepath.Join(state, "logs")
	}
	if d.Ledger == nil {
		d.Ledger, err = NewLedger("")
	}
	return d, err
}

func (d Deps) location() *time.Location {
	loc, err := time.LoadLocation(d.Settings.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (d Deps) english() bool {
	return d.Settings.Language == "en"
}
