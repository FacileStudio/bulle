package assistant

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	sentWindow = "3d"
	maxSent    = 8
)

// SentItem is one mail the user sent recently, kept as context for a digest:
// it says what they are waiting on, never what to notify.
type SentItem struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	At      string `json:"at"`
}

type sentDoc struct {
	Emails []struct {
		Subject string `json:"subject"`
		Date    string `json:"date"`
		To      []struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"to_addresses"`
	} `json:"emails"`
}

// RecentSent lists the newest mails the user sent in the last three days,
// across the picked accounts. A failure is returned with whatever was read so
// far; callers treat it as missing context, not as an error to stop on.
func RecentSent(ctx context.Context, d Deps) ([]SentItem, error) {
	src := &MailSource{Run: d.Run, Settings: d.Settings}
	var accounts []mailAccount
	if err := runJSON(ctx, d.Run, 0, &accounts, "courrier", "accounts", "--json"); err != nil {
		return nil, err
	}
	var out []SentItem
	for _, a := range accounts {
		if !src.wantsAccount(a.ID) {
			continue
		}
		got, err := sentOf(ctx, d, a.ID)
		if err != nil {
			return out, fmt.Errorf("sent mail of account %d: %w", a.ID, err)
		}
		out = append(out, got...)
	}
	if len(out) > maxSent {
		out = out[:maxSent]
	}
	return out, nil
}

func sentOf(ctx context.Context, d Deps, account int64) ([]SentItem, error) {
	var doc sentDoc
	id := strconv.FormatInt(account, 10)
	args := []string{"courrier", "list", "sent", "--account", id, "--since", sentWindow, "--limit", strconv.Itoa(maxSent), "--json"}
	if err := runJSON(ctx, d.Run, 0, &doc, args...); err != nil {
		return nil, err
	}
	var items []SentItem
	for _, e := range doc.Emails {
		var to []string
		for _, r := range e.To {
			to = append(to, firstNonEmpty(r.Name, r.Email))
		}
		at := ""
		if t, err := time.Parse(time.RFC3339, e.Date); err == nil {
			at = t.Format("2006-01-02")
		}
		items = append(items, SentItem{To: strings.Join(to, ", "), Subject: e.Subject, At: at})
	}
	return items, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
