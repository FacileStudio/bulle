package assistant

import "fmt"

func (m *MailSource) items(rows []TriageItem) []Item {
	drop := append(append([]string{}, m.Settings.DropSenders...), LoadDropSenders(m.StateDir)...)
	senders := map[string]string{}
	out := make([]Item, 0, len(rows))
	for _, t := range rows {
		it := m.rowItem(t, drop)
		senders[it.Key()] = t.FromAddress
		out = append(out, it)
	}
	if err := RecordMailSenders(m.StateDir, senders); err != nil && m.Log != nil {
		fmt.Fprintf(m.Log, "assistant: record mail senders: %v\n", err)
	}
	return out
}

func (m *MailSource) rowItem(t TriageItem, drop []string) Item {
	it := Item{
		Source:  SourceMail,
		ID:      t.ThreadID,
		Title:   t.Subject,
		Who:     t.FromName,
		Account: t.Account,
		When:    t.Date,
		Count:   t.MessageCount,
		Class:   Class(t.Class),
		Reason:  t.Reason,
	}
	if it.Who == "" {
		it.Who = t.FromAddress
	}
	if MatchesDrop(t.FromAddress, drop) {
		it.Class, it.Reason = ClassBulk, "drop-sender"
	}
	if IsVIP(t.FromAddress, m.Settings.VIP) {
		it.Class, it.Reason = ClassVIP, "vip"
	}
	return it
}
