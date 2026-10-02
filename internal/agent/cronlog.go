package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

// cronStatus is the outcome appended to a job's delivery log.
type cronStatus string

const (
	cronOK     cronStatus = "ok"
	cronFailed cronStatus = "failed"
)

// validateDelivery checks a job's delivery target before any billed run, so a
// bad delivery: value fails fast instead of after the full run.
func validateDelivery(delivery string) error {
	if delivery == "" || strings.HasPrefix(delivery, "file:") {
		return nil
	}
	return fmt.Errorf("unknown delivery %q: want file:<dir>", delivery)
}

func appendCronLog(delivery, name string, status cronStatus, text string) error {
	if err := validateDelivery(delivery); err != nil {
		return err
	}
	dir := ""
	if rest, ok := strings.CutPrefix(delivery, "file:"); ok {
		dir = rest
	}
	if dir == "" {
		return nil
	}
	dir = expandHome(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating delivery dir %s: %w", dir, err)
	}
	now := time.Now()
	line := fmt.Sprintf("=== %s %s ===\n", now.Format(time.RFC3339), status)
	day := appendFile(filepath.Join(dir, name+"-"+now.Format("2006-01-02")+".md"), "\n"+line+text)
	health := appendFile(filepath.Join(dir, name+".log"), line)
	return errors.Join(day, health)
}

func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	return nil
}

// cronUsage is the billed part of a run record. It mirrors nacelle.Usage so
// the JSON keys stay snake_case like the rest of the record.
type cronUsage struct {
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	Cost                float64 `json:"cost"`
}

// cronRun is one JSON line in the central cron run log: what ran, what it
// cost, and how much context it ended on.
type cronRun struct {
	Name               string    `json:"name"`
	Started            string    `json:"started"`
	DurationMs         int64     `json:"duration_ms"`
	Status             string    `json:"status"`
	Model              string    `json:"model"`
	ToolCalls          int       `json:"tool_calls"`
	FinalContextTokens int64     `json:"final_context_tokens"`
	Compactions        int       `json:"compactions"`
	Usage              cronUsage `json:"usage"`
	Error              string    `json:"error,omitempty"`
}

// cronStatePath is the central cron run log: one JSON line per run, every
// job, regardless of delivery. Under XDG_STATE_HOME when set.
func cronStatePath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "bulle", "cron.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving state dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "bulle", "cron.jsonl"), nil
}

func appendCronRun(rec cronRun) error {
	path, err := cronStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cron state dir: %w", err)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding cron run record: %w", err)
	}
	return appendFile(path, string(line)+"\n")
}

// record turns the run's measurements into a central-log line and reports the
// status the delivery log should carry.
func (s runStats) record(job settings.CronJob, model string, started time.Time, runErr error) (cronRun, cronStatus) {
	rec := cronRun{
		Name:               job.Name,
		Started:            started.Format(time.RFC3339),
		DurationMs:         time.Since(started).Milliseconds(),
		Status:             string(cronOK),
		Model:              model,
		ToolCalls:          s.ToolCalls,
		FinalContextTokens: s.FinalContextTokens,
		Compactions:        s.Compactions,
		Usage: cronUsage{
			InputTokens:         s.InputTokens,
			OutputTokens:        s.OutputTokens,
			CacheReadTokens:     s.CacheReadTokens,
			CacheCreationTokens: s.CacheCreationTokens,
			Cost:                s.Cost,
		},
	}
	if runErr == nil {
		return rec, cronOK
	}
	rec.Status = string(cronFailed)
	rec.Error = runErr.Error()
	return rec, cronFailed
}
