package assistant

import (
	"sort"
	"time"
)

const (
	maxDigestItems = 8
	maxUnknown     = 5
)

// DigestItem is one line the model may write about.
type DigestItem struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Who   string `json:"who,omitempty"`
	At    string `json:"at,omitempty"`
}

// Payload is the only thing the model sees: at most 8 ranked items, the names
// of unknown senders, and a bulk count the prompt forbids mentioning.
type Payload struct {
	Digest  string       `json:"digest"`
	Date    string       `json:"date"`
	Items   []DigestItem `json:"items"`
	Unknown []string     `json:"unknown_senders,omitempty"`
	Bulk    int          `json:"bulk_never_mention"`
	listed  []Item
}

func buildPayload(name string, items []Item, now time.Time) Payload {
	p := Payload{Digest: name, Date: now.Format("2006-01-02")}
	var ranked []Item
	ranked = append(ranked, todayEvents(items, now, name == "morning")...)
	ranked = append(ranked, tasksIn(items, now, "overdue")...)
	ranked = append(ranked, tasksIn(items, now, "today")...)
	ranked = append(ranked, mailOf(items, ClassVIP)...)
	ranked = append(ranked, mailOf(items, ClassKnown)...)
	if len(ranked) > maxDigestItems {
		ranked = ranked[:maxDigestItems]
	}
	for _, it := range ranked {
		p.Items = append(p.Items, toDigestItem(it, now))
	}
	p.listed = ranked
	for _, it := range mailOf(items, ClassUnknownHuman) {
		if len(p.Unknown) < maxUnknown && !contains(p.Unknown, it.Who) {
			p.Unknown = append(p.Unknown, it.Who)
			p.listed = append(p.listed, it)
		}
	}
	p.Bulk = len(mailOf(items, ClassBulk))
	return p
}

func todayEvents(items []Item, now time.Time, all bool) []Item {
	var out []Item
	for _, it := range items {
		if it.Source != SourceCalendar {
			continue
		}
		w := it.When.In(now.Location())
		sameDay := w.Format("2006-01-02") == now.Format("2006-01-02")
		if sameDay && (all || !w.Before(now)) {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].When.Before(out[j].When) })
	return out
}

func tasksIn(items []Item, now time.Time, phase string) []Item {
	var out []Item
	for _, it := range items {
		if it.Source == SourceTask && phaseOf(it, now) == phase {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Due.Before(out[j].Due) })
	return out
}

func mailOf(items []Item, class Class) []Item {
	var out []Item
	for _, it := range items {
		if it.Source == SourceMail && it.Class == class {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].When.After(out[j].When) })
	return out
}

func toDigestItem(it Item, now time.Time) DigestItem {
	d := DigestItem{Title: it.Title, Who: it.Who}
	switch it.Source {
	case SourceCalendar:
		d.Kind, d.At = "event", it.When.In(now.Location()).Format("15:04")
	case SourceMail:
		d.Kind = "mail"
	default:
		d.Kind = "due_today"
		if phaseOf(it, now) == "overdue" {
			d.Kind = "overdue"
		}
	}
	return d
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
