package assistant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

const triageUnsupported = "does not support triage"

// MailSource reads unread mail through courrier. It prefers `courrier triage`
// and falls back to per-account inbox calls with local sender rules when the
// server is too old.
type MailSource struct {
	Run      Runner
	Settings settings.Assistant
	Since    string
	StateDir string
	Timeout  time.Duration
	Log      io.Writer
	once     sync.Once
}

// Fetch returns the unread mail conversations as Items. A VIP sender wins over
// every class, bulk included.
func (m *MailSource) Fetch(ctx context.Context) ([]Item, error) {
	items, err := m.fetchTriage(ctx)
	var ce *CmdError
	if errors.As(err, &ce) && strings.Contains(ce.Stderr, triageUnsupported) {
		m.once.Do(m.logFallback)
		return m.fetchFallback(ctx)
	}
	return items, err
}

func (m *MailSource) logFallback() {
	w := m.Log
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintln(w, "assistant: courrier triage unsupported, using inbox fallback with local sender rules")
}

func (m *MailSource) fetchTriage(ctx context.Context) ([]Item, error) {
	args := []string{"courrier", "triage", "--all", "--unread"}
	if m.Since != "" {
		args = append(args, "--since", m.Since)
	}
	args = append(args, "--json")
	var doc Triage
	if err := runJSON(ctx, m.Run, m.Timeout, &doc, args...); err != nil {
		return nil, err
	}
	var rows []TriageItem
	for _, t := range doc.Items {
		if m.wantsAccount(t.AccountID) {
			rows = append(rows, t)
		}
	}
	return m.items(rows), nil
}

func (m *MailSource) wantsAccount(id int64) bool {
	return pickedID(m.Settings.Sources.MailAccounts, int(id))
}
