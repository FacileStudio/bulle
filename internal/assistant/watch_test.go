package assistant

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

type rig struct {
	t       *testing.T
	now     time.Time
	triage  string
	events  string
	sent    []chat.IPCRequest
	sendErr error
	model   string
	modelN  int
	d       Deps
}

func (r *rig) run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	switch {
	case strings.HasPrefix(key, "courrier triage"):
		return []byte(r.triage), nil
	case strings.HasPrefix(key, "agenda calendars"):
		return []byte(`[{"id":1,"name":"Perso"}]`), nil
	case strings.HasPrefix(key, "agenda events"):
		return []byte(r.events), nil
	case strings.HasPrefix(key, "opus workspace"):
		return []byte(`[]`), nil
	}
	return nil, fmt.Errorf("unexpected command: %s", key)
}

func (r *rig) Complete(context.Context, string) (string, error) {
	r.modelN++
	return r.model, nil
}

func newRig(t *testing.T, at string) *rig {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	loc, _ := time.LoadLocation("Europe/Paris")
	now, err := time.ParseInLocation("2006-01-02 15:04", at, loc)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, now: now, triage: `{"items":[]}`, events: `[]`}
	ledger, err := NewLedger(t.TempDir() + "/ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	r.d = Deps{
		Now:      func() time.Time { return r.now },
		Run:      r.run,
		Ledger:   ledger,
		StateDir: t.TempDir(),
		LogDir:   t.TempDir(),
		Sleep:    func(time.Duration) {},
		Complete: r,
		Send: func(_ context.Context, req chat.IPCRequest) error {
			r.sent = append(r.sent, req)
			return r.sendErr
		},
	}
	return r
}

const knownMail = `{"items":[{"account_id":1,"account":"a","thread_id":"t1","subject":"Devis",
"from_address":"marie@x.fr","from_name":"Marie","date":"2026-10-30T09:50:00Z","message_count":1,
"class":"known"}]}`

func TestWatchQuietHoursSendsNothing(t *testing.T) {
	r := newRig(t, "2026-10-30 03:00")
	r.triage = knownMail
	res, err := Watch(context.Background(), r.d)
	if err != nil || len(r.sent) != 0 || res.Delivery != DeliverySuppressed {
		t.Fatalf("err=%v sent=%d delivery=%q", err, len(r.sent), res.Delivery)
	}
	if !LastWatch(r.d.StateDir).IsZero() {
		t.Error("quiet run must not advance the watch time")
	}
}

func TestWatchQuietUntilFile(t *testing.T) {
	r := newRig(t, "2026-10-30 10:00")
	r.triage = knownMail
	until := r.now.Add(time.Hour).UTC().Format(time.RFC3339)
	if err := writeAtomic(r.d.StateDir+"/quiet.json", []byte(`{"until":"`+until+`"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := Watch(context.Background(), r.d); err != nil || len(r.sent) != 0 {
		t.Fatalf("err=%v sent=%d", err, len(r.sent))
	}
}

func TestWatchInterruptsOnceThenLedgerSilences(t *testing.T) {
	r := newRig(t, "2026-10-30 10:00")
	r.triage = knownMail
	res, err := Watch(context.Background(), r.d)
	if err != nil || len(r.sent) != 1 || res.Delivery != DeliveryOK {
		t.Fatalf("err=%v sent=%d delivery=%q", err, len(r.sent), res.Delivery)
	}
	req := r.sent[0]
	if req.Notify == nil || !*req.Notify || !req.Record || len(req.Items) != 1 || req.Items[0] != "mail:t1" {
		t.Errorf("bad request: %+v", req)
	}
	if !strings.Contains(req.Text, "Marie") {
		t.Errorf("text = %q", req.Text)
	}
	r.now = r.now.Add(15 * time.Minute)
	if _, err := Watch(context.Background(), r.d); err != nil || len(r.sent) != 1 {
		t.Fatalf("second run sent=%d err=%v", len(r.sent), err)
	}
	if r.modelN != 0 {
		t.Errorf("watch called the model %d times", r.modelN)
	}
}

func TestWatchNoTriggerNoMessage(t *testing.T) {
	r := newRig(t, "2026-10-30 10:00")
	r.triage = strings.Replace(knownMail, `"known"`, `"unknown_human"`, 1)
	res, err := Watch(context.Background(), r.d)
	if err != nil || len(r.sent) != 0 || res.Delivery != "" || r.modelN != 0 {
		t.Fatalf("err=%v sent=%d delivery=%q model=%d", err, len(r.sent), res.Delivery, r.modelN)
	}
}

func TestWatchEventWithinAnHour(t *testing.T) {
	r := newRig(t, "2026-10-30 10:00")
	r.events = `[{"id":7,"title":"Standup","start_at":"2026-10-30T09:30:00Z","status":"confirmed"}]`
	if _, err := Watch(context.Background(), r.d); err != nil || len(r.sent) != 1 {
		t.Fatalf("err=%v sent=%d", err, len(r.sent))
	}
	if !strings.Contains(r.sent[0].Text, "Dans 30 min : Standup") {
		t.Errorf("text = %q", r.sent[0].Text)
	}
}

func TestWatchFailedSendFallsBackToFile(t *testing.T) {
	r := newRig(t, "2026-10-30 10:00")
	r.triage = knownMail
	r.sendErr = fmt.Errorf("socket down")
	res, err := Watch(context.Background(), r.d)
	if err != nil || res.Delivery != DeliveryFailed || len(r.sent) != 3 {
		t.Fatalf("err=%v delivery=%q attempts=%d", err, res.Delivery, len(r.sent))
	}
	if !r.d.Ledger.ShouldSurface(res.Surfaced[0], r.now) {
		t.Error("failed delivery must leave the item unsurfaced")
	}
}
