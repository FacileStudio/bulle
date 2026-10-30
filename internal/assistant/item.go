// Package assistant decides what Yann should hear about and when. Plain Go
// fetches the data, filters it and checks the ledger; the model only writes
// the final lines from what is left.
package assistant

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// Source names where an Item came from.
type Source string

const (
	SourceMail     Source = "mail"
	SourceCalendar Source = "calendar"
	SourceTask     Source = "task"
)

// Class says how much a mail deserves attention. Courrier decides bulk, known
// and unknown_human; the assistant adds vip from its own list, and a vip wins
// over every other class.
type Class string

const (
	ClassVIP          Class = "vip"
	ClassKnown        Class = "known"
	ClassUnknownHuman Class = "unknown_human"
	ClassBulk         Class = "bulk"
)

// Item is one thing the assistant may surface: a mail conversation, a calendar
// event or a task. Every source returns []Item so the ledger and the digest
// never care where one came from.
type Item struct {
	Source  Source    `json:"source"`
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Who     string    `json:"who,omitempty"`
	Account string    `json:"account,omitempty"`
	When    time.Time `json:"when,omitempty"`
	Due     time.Time `json:"due,omitempty"`
	Status  string    `json:"status,omitempty"`
	Count   int       `json:"count,omitempty"`
	Class   Class     `json:"class,omitempty"`
	Reason  string    `json:"reason,omitempty"`
}

// Key is the stable ledger key of the item: source and id, never the title.
func (i Item) Key() string {
	return string(i.Source) + ":" + i.ID
}

// Fingerprint changes when the item changed in a way worth a second mention:
// its status, its due date, how many messages the thread holds, or its start
// time. A title edit alone does not change it.
func (i Item) Fingerprint() string {
	parts := []string{
		i.Status,
		strconv.Itoa(i.Count),
		i.Due.UTC().Format(time.RFC3339),
		i.When.UTC().Format(time.RFC3339),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}

// TriageItem is one conversation as `courrier triage --json` returns it. The
// contract lives in docs/plan-courrier-triage.md.
type TriageItem struct {
	AccountID    int64     `json:"account_id"`
	Account      string    `json:"account"`
	ThreadID     string    `json:"thread_id"`
	EmailID      int64     `json:"email_id"`
	Subject      string    `json:"subject"`
	FromAddress  string    `json:"from_address"`
	FromName     string    `json:"from_name"`
	Date         time.Time `json:"date"`
	MessageCount int       `json:"message_count"`
	UnreadCount  int       `json:"unread_count"`
	Class        string    `json:"class"`
	Reason       string    `json:"reason"`
}

// Triage is the whole `courrier triage --json` document. Counts covers the
// full filtered set, not only the returned page.
type Triage struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Total       int            `json:"total"`
	Counts      map[string]int `json:"counts"`
	Items       []TriageItem   `json:"items"`
	Page        int            `json:"page"`
	Limit       int            `json:"limit"`
}
