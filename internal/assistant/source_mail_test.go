package assistant

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

const triageKey = "courrier triage --all --unread --since 2h --json"

func classes(items []Item) map[string]Class {
	out := map[string]Class{}
	for _, it := range items {
		out[it.ID] = it.Class
	}
	return out
}

type triageCase struct {
	name     string
	settings settings.Assistant
	want     map[string]Class
}

var triageCases = []triageCase{
	{"courrier classes kept", settings.Assistant{}, map[string]Class{
		"20261023204141.40f2@notify.railway.app": ClassBulk,
		"cajtkw-remarks@mail.example.test":       ClassKnown,
		"intro-request@mail.example.test":        ClassUnknownHuman,
		"bank-statement@mail.example.test":       ClassBulk,
	}},
	{"vip overrides bulk", settings.Assistant{VIP: []string{"boss@example.org"}}, map[string]Class{
		"20261023204141.40f2@notify.railway.app": ClassBulk,
		"cajtkw-remarks@mail.example.test":       ClassKnown,
		"intro-request@mail.example.test":        ClassUnknownHuman,
		"bank-statement@mail.example.test":       ClassVIP,
	}},
	{"account filter", settings.Assistant{Sources: settings.AssistantSources{MailAccounts: []int{3}}}, map[string]Class{
		"intro-request@mail.example.test":  ClassUnknownHuman,
		"bank-statement@mail.example.test": ClassBulk,
	}},
}

func checkClasses(t *testing.T, items []Item, want map[string]Class) {
	t.Helper()
	got := classes(items)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for id, class := range want {
		if got[id] != class {
			t.Errorf("%s: got %q, want %q", id, got[id], class)
		}
	}
}

func TestMailTriage(t *testing.T) {
	for _, c := range triageCases {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t, map[string]string{triageKey: "triage.json"})
			src := &MailSource{Run: f.run, Settings: c.settings, Since: "2h"}
			items, err := src.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			checkClasses(t, items, c.want)
		})
	}
}

func TestMailItemMapping(t *testing.T) {
	f := newFake(t, map[string]string{triageKey: "triage.json"})
	src := &MailSource{Run: f.run, Since: "2h"}
	items, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	alice := items[1]
	if alice.Source != SourceMail || alice.Who != "Alice Martin" || alice.Count != 3 ||
		alice.Account != "me@example.test" || alice.Reason != "in-sent-thread" || alice.When.IsZero() {
		t.Errorf("bad mapping: %+v", alice)
	}
	if items[2].Who != "stranger@example.net" {
		t.Errorf("empty name should fall back to address: %q", items[2].Who)
	}
}

func TestMailFallback(t *testing.T) {
	f := newFake(t, map[string]string{
		"courrier accounts --json":                   "accounts.json",
		"courrier inbox --account 1 --unread --json": "inbox_account1.json",
		"courrier inbox --account 3 --unread --json": "inbox_account3.json",
		"courrier triage --all --unread --json":      "triage.json",
	})
	f.fails["courrier triage --all --unread --json"] = &CmdError{
		Stderr: "this Courrier instance does not support triage; upgrade the server",
		Err:    errors.New("exit status 1"),
	}
	var log bytes.Buffer
	src := &MailSource{
		Run:      f.run,
		Log:      &log,
		Settings: settings.Assistant{DropSenders: []string{"shop.example.net"}, VIP: []string{"ALICE@example.org"}},
	}
	want := map[string]Class{
		"20261023204141.40f2@notify.railway.app": ClassBulk,
		"cajtkw-remarks@mail.example.test":       ClassVIP,
		"promo@mail.example.test":                ClassBulk,
	}
	for range 2 {
		items, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		checkClasses(t, items, want)
	}
	if n := strings.Count(log.String(), "fallback"); n != 1 {
		t.Errorf("fallback logged %d times, want 1", n)
	}
}

func TestMailOtherErrorsPropagate(t *testing.T) {
	f := newFake(t, nil)
	f.fails["courrier triage --all --unread --json"] = &CmdError{Stderr: "boom", Err: errors.New("exit status 1")}
	src := &MailSource{Run: f.run}
	if _, err := src.Fetch(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}
