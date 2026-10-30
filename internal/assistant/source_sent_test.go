package assistant

import (
	"context"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestRecentSentReadsThePickedAccountsSentFolder(t *testing.T) {
	f := newFake(t, map[string]string{
		"courrier accounts --json": `[{"id":1,"email":"a@x.test"},{"id":2,"email":"b@x.test"}]`,
		"courrier list sent --account 1 --since 3d --limit 8 --json": `{"emails":[` +
			`{"subject":"Devis","date":"2026-10-29T10:00:00Z","to_addresses":[{"name":"Alice","email":"alice@x.test"}]},` +
			`{"subject":"Re: dates","date":"2026-10-28T09:00:00Z","to_addresses":[{"name":"","email":"bob@x.test"}]}]}`,
	})
	var cfg settings.Assistant
	cfg.Sources.MailAccounts = []int{1}
	got, err := RecentSent(context.Background(), Deps{Run: f.run, Settings: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].To != "Alice" || got[0].At != "2026-10-29" || got[1].To != "bob@x.test" {
		t.Errorf("sent = %+v", got)
	}
	for _, call := range f.calls {
		if call == "courrier list sent --account 2 --since 3d --limit 8 --json" {
			t.Error("an account that is not picked must not be read")
		}
	}
}

func TestDigestPromptCarriesTheSentContext(t *testing.T) {
	p := Payload{Digest: "morning", Sent: []SentItem{{To: "Alice", Subject: "Devis", At: "2026-10-29"}}}
	prompt, err := digestPrompt(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "sent_recently") || !strings.Contains(prompt, "Devis") {
		t.Errorf("prompt = %q", prompt)
	}
}
