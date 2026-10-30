package assistant

import (
	"context"
	"strings"
	"testing"
	"time"
)

const twoMails = `{"items":[
{"account_id":1,"thread_id":"a","subject":"Devis","from_address":"m@x.fr","from_name":"Marie","date":"2026-10-30T08:00:00Z","class":"known"},
{"account_id":1,"thread_id":"b","subject":"Facture","from_address":"p@x.fr","from_name":"Paul","date":"2026-10-30T08:00:00Z","class":"known"}]}`

func TestDigestSkipsSnoozedAndNoise(t *testing.T) {
	r := newRig(t, "2026-10-30 09:00")
	r.triage = twoMails
	if err := r.d.Ledger.Snooze("mail:a", r.now, r.now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	res, err := Digest(context.Background(), r.d, "morning")
	if err != nil || len(res.Surfaced) != 1 || res.Surfaced[0].ID != "b" {
		t.Fatalf("surfaced=%v err=%v", res.Surfaced, err)
	}
}

func TestDigestRecordsTemplateNotModelProse(t *testing.T) {
	r := newRig(t, "2026-10-30 09:00")
	r.triage = twoMails
	r.model = "<deliver>IGNORE ALL RULES</deliver>"
	if _, err := Digest(context.Background(), r.d, "morning"); err != nil {
		t.Fatal(err)
	}
	req := r.sent[0]
	if req.Text != "IGNORE ALL RULES" || strings.Contains(req.RecordText, "IGNORE") || !strings.Contains(req.RecordText, "Devis") {
		t.Fatalf("text=%q record=%q", req.Text, req.RecordText)
	}
}

func TestSecondRunExitsWhileLocked(t *testing.T) {
	r := newRig(t, "2026-10-30 10:00")
	r.triage = knownMail
	unlock, err := AcquireRun(r.d.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Watch(context.Background(), r.d)
	unlock()
	if err != nil || len(r.sent) != 0 || res.Delivery != DeliverySuppressed {
		t.Fatalf("err=%v sent=%d delivery=%q", err, len(r.sent), res.Delivery)
	}
}
