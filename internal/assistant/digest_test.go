package assistant

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDigestUsesModelText(t *testing.T) {
	r := newRig(t, "2026-10-30 09:00")
	r.triage = knownMail
	r.model = "bla <deliver>Marie t'a écrit au sujet du devis.</deliver>"
	res, err := Digest(context.Background(), r.d, "morning")
	if err != nil || len(r.sent) != 1 || !res.ModelCalled {
		t.Fatalf("err=%v sent=%d", err, len(r.sent))
	}
	req := r.sent[0]
	if req.Text != "Marie t'a écrit au sujet du devis." || req.Notify == nil || *req.Notify || !req.Record {
		t.Errorf("bad request: %+v", req)
	}
}

func TestDigestFallsBackToTemplate(t *testing.T) {
	for name, model := range map[string]string{
		"no tag":   "Voici ton résumé",
		"too long": "<deliver>" + strings.Repeat("x", 1300) + "</deliver>",
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, "2026-10-30 09:00")
			r.triage = knownMail
			r.model = model
			if _, err := Digest(context.Background(), r.d, "morning"); err != nil || len(r.sent) != 1 {
				t.Fatalf("err=%v sent=%d", err, len(r.sent))
			}
			if got := r.sent[0].Text; !strings.Contains(got, "Mail de Marie : Devis") {
				t.Errorf("template not used: %q", got)
			}
		})
	}
}

func TestDigestEveningEndsWithDebrief(t *testing.T) {
	r := newRig(t, "2026-10-30 20:00")
	res, err := Digest(context.Background(), r.d, "evening")
	if err != nil || len(r.sent) != 1 || res.ModelCalled {
		t.Fatalf("err=%v sent=%d model=%v", err, len(r.sent), res.ModelCalled)
	}
	if !strings.HasSuffix(r.sent[0].Text, debriefInvite) {
		t.Errorf("text = %q", r.sent[0].Text)
	}
}

func TestDigestWeekendRules(t *testing.T) {
	r := newRig(t, "2026-10-31 14:00")
	r.triage = knownMail
	if res, err := Digest(context.Background(), r.d, "midday"); err != nil || len(r.sent) != 0 || res.Delivery != DeliverySuppressed {
		t.Fatalf("midday on saturday: err=%v sent=%d", err, len(r.sent))
	}
	r.now = r.now.Add(-5 * time.Hour)
	if _, err := Digest(context.Background(), r.d, "morning"); err != nil || len(r.sent) != 1 {
		t.Fatalf("morning on saturday: err=%v sent=%d", err, len(r.sent))
	}
}

func TestDigestPayloadHidesBulk(t *testing.T) {
	r := newRig(t, "2026-10-30 09:00")
	r.triage = `{"items":[
{"account_id":1,"thread_id":"b","subject":"Promo","from_name":"Shop","date":"2026-10-30T08:00:00Z","class":"bulk"},
{"account_id":1,"thread_id":"u","subject":"Salut","from_name":"Paul","date":"2026-10-30T08:00:00Z","class":"unknown_human"}]}`
	r.model = "<deliver>ok</deliver>"
	res, err := Digest(context.Background(), r.d, "morning")
	if err != nil {
		t.Fatal(err)
	}
	p := buildPayload("morning", []Item{{Source: SourceMail, ID: "b", Class: ClassBulk}, {Source: SourceMail, ID: "u", Class: ClassUnknownHuman, Who: "Paul"}}, r.now)
	if p.Bulk != 1 || len(p.Items) != 0 || len(p.Unknown) != 1 || len(res.Surfaced) != 1 {
		t.Errorf("payload = %+v surfaced=%d", p, len(res.Surfaced))
	}
}

func TestDigestUnknownName(t *testing.T) {
	r := newRig(t, "2026-10-30 09:00")
	if _, err := Digest(context.Background(), r.d, "noon"); err == nil {
		t.Fatal("want error")
	}
}
