package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Completer is the model call Consolidate needs.
type Completer interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// Options tunes Consolidate.
type Options struct {
	JournalPath string
}

// Report lists what a consolidation changed.
type Report struct {
	Added   []string
	Dropped []string
	Refused []string
	Backup  string
}

type plan struct {
	Add  []string `json:"add"`
	Drop []string `json:"drop"`
}

// Consolidate asks the model which durable facts to add to MEMORY.md and which
// lines to drop, validates the answer in code, writes backups/MEMORY.md.<date>
// and applies it. Unparseable output changes nothing and returns an error.
func (s *Store) Consolidate(ctx context.Context, c Completer, now time.Time, opt Options) (Report, error) {
	day := now.AddDate(0, 0, -1).Format("2006-01-02")
	notes, err := s.readFile(s.path("notes", day+".md"))
	if err != nil {
		return Report{}, err
	}
	journal := ""
	if opt.JournalPath != "" {
		journal, _ = s.readFile(opt.JournalPath)
	}
	if strings.TrimSpace(notes+journal) == "" {
		return Report{}, nil
	}
	current, err := s.readFile(s.path(MemoryFile))
	if err != nil {
		return Report{}, err
	}
	raw, err := c.Complete(ctx, buildPrompt(current, notes, journal))
	if err != nil {
		return Report{}, err
	}
	p, err := parsePlan(raw)
	if err != nil {
		return Report{}, err
	}
	return s.applyPlan(p, now.Format("2006-01-02"))
}

func buildPrompt(current, notes, journal string) string {
	return fmt.Sprintf(`You maintain a long-term memory file. Reply with strict JSON only, no prose:
{"add": ["fact", ...], "drop": ["exact existing line", ...]}
Add only durable facts worth keeping for months. Drop only lines that are stale, copied exactly.
Limit: %d characters in total.

CURRENT MEMORY:
%s
YESTERDAY'S NOTES:
%s
JOURNAL:
%s`, MemoryCap, current, notes, journal)
}

func parsePlan(raw string) (plan, error) {
	var p plan
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &p); err != nil {
		return plan{}, fmt.Errorf("memory: model output is not the expected JSON: %w", err)
	}
	return p, nil
}

func (s *Store) applyPlan(p plan, date string) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.readFile(s.path(MemoryFile))
	if err != nil {
		return Report{}, err
	}
	kept, dropped := dropExact(splitLines(cur), p.Drop)
	added, merged := addFacts(kept, p.Add)
	rep := Report{Added: added, Dropped: dropped}
	if utf8.RuneCountInString(strings.TrimSpace(joinLines(merged))) > MemoryCap {
		rep.Added, rep.Refused, merged = nil, added, kept
	}
	if len(rep.Added) == 0 && len(dropped) == 0 {
		return rep, nil
	}
	rep.Backup = s.path("backups", MemoryFile+"."+date)
	if err := s.keepBackup(date, cur); err != nil {
		return rep, err
	}
	return rep, s.writeAtomic(s.path(MemoryFile), []byte(joinLines(merged)))
}
