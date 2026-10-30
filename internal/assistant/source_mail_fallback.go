package assistant

import (
	"context"
	"strconv"
)

type mailAccount struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type inboxDoc struct {
	Emails []TriageItem `json:"emails"`
}

func (m *MailSource) fetchFallback(ctx context.Context) ([]Item, error) {
	var accounts []mailAccount
	if err := runJSON(ctx, m.Run, m.Timeout, &accounts, "courrier", "accounts", "--json"); err != nil {
		return nil, err
	}
	var rows []TriageItem
	for _, a := range accounts {
		if !m.wantsAccount(a.ID) {
			continue
		}
		got, err := m.inbox(ctx, a)
		if err != nil {
			return nil, err
		}
		rows = append(rows, got...)
	}
	return m.items(rows), nil
}

func (m *MailSource) inbox(ctx context.Context, a mailAccount) ([]TriageItem, error) {
	var doc inboxDoc
	id := strconv.FormatInt(a.ID, 10)
	if err := runJSON(ctx, m.Run, m.Timeout, &doc, "courrier", "inbox", "--account", id, "--unread", "--json"); err != nil {
		return nil, err
	}
	for i := range doc.Emails {
		e := &doc.Emails[i]
		class, reason := ClassifySender(e.FromAddress, m.Settings.DropSenders)
		e.Class, e.Reason = string(class), reason
		e.Account = a.Email
	}
	return doc.Emails, nil
}
