package chat

import (
	"encoding/json"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const selfMXID = "@bulle:example.org"

const roomID = "!room:example.org"

// syncFixture is one realistic /sync payload: a room timeline holding the
// shapes the adapter must sort through, from a plain human message to the
// membership and reaction noise every real timeline carries.
const syncFixture = `{
  "next_batch": "s72595_4483_1934",
  "rooms": {
    "join": {
      "!room:example.org": {
        "timeline": {
          "events": [
            {
              "type": "m.room.message",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "event_id": "$plain:example.org",
              "origin_server_ts": 1700000000000,
              "content": {"msgtype": "m.text", "body": "hello bulle"}
            },
            {
              "type": "m.room.message",
              "room_id": "!room:example.org",
              "sender": "@bulle:example.org",
              "event_id": "$self:example.org",
              "origin_server_ts": 1700000001000,
              "content": {"msgtype": "m.text", "body": "my own reply"}
            },
            {
              "type": "m.room.message",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "event_id": "$edit:example.org",
              "origin_server_ts": 1700000002000,
              "content": {
                "msgtype": "m.text",
                "body": "* corrected",
                "m.relates_to": {"rel_type": "m.replace", "event_id": "$plain:example.org"},
                "m.new_content": {"msgtype": "m.text", "body": "corrected"}
              }
            },
            {
              "type": "m.room.message",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "event_id": "$notice:example.org",
              "origin_server_ts": 1700000003000,
              "content": {"msgtype": "m.notice", "body": "deploy finished"}
            },
            {
              "type": "m.room.message",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "event_id": "$image:example.org",
              "origin_server_ts": 1700000004000,
              "content": {"msgtype": "m.image", "url": "mxc://example.org/abc"}
            },
            {
              "type": "m.room.message",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "event_id": "$blank:example.org",
              "origin_server_ts": 1700000005000,
              "content": {"msgtype": "m.text", "body": "   "}
            },
            {
              "type": "m.reaction",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "event_id": "$reaction:example.org",
              "origin_server_ts": 1700000006000,
              "content": {
                "m.relates_to": {"rel_type": "m.annotation", "event_id": "$plain:example.org", "key": "+1"}
              }
            },
            {
              "type": "m.room.member",
              "room_id": "!room:example.org",
              "sender": "@alice:example.org",
              "state_key": "@alice:example.org",
              "event_id": "$member:example.org",
              "origin_server_ts": 1700000007000,
              "content": {"membership": "join"}
            }
          ]
        }
      }
    }
  }
}`

// want is what messageFrom must produce for one fixture event it accepts. A
// separate table states the refusals, since those assert one thing.
type want struct {
	sender string
	room   string
	event  string
	text   string
	at     int64
}

// TestMessageFromRefuses is the half of the rule that keeps the loop from
// being a self-reply: the bot's own message, an edit, media with no
// instruction in it, and the membership and reaction noise every real
// timeline carries.
func TestMessageFromRefuses(t *testing.T) {
	events := fixtureEvents(t)
	refused := map[string]string{
		"$self:example.org":     "the bot's own message",
		"$edit:example.org":     "an edit carrying m.replace",
		"$image:example.org":    "an image with no body",
		"$blank:example.org":    "a whitespace-only body",
		"$reaction:example.org": "a reaction",
		"$member:example.org":   "a membership change",
	}
	for eventID, name := range refused {
		t.Run(name, func(t *testing.T) {
			evt, ok := events[eventID]
			if !ok {
				t.Fatalf("fixture has no event %s", eventID)
			}
			if _, accepted := messageFrom(evt, id.UserID(selfMXID)); accepted {
				t.Errorf("%s was accepted, want refused", eventID)
			}
		})
	}
}

func TestMessageFromAccepts(t *testing.T) {
	events := fixtureEvents(t)
	cases := map[string]want{
		"$plain:example.org":  {sender: "@alice:example.org", room: roomID, event: "$plain:example.org", text: "hello bulle", at: 1700000000000},
		"$notice:example.org": {sender: "@alice:example.org", room: roomID, event: "$notice:example.org", text: "deploy finished", at: 1700000003000},
	}
	for eventID, w := range cases {
		t.Run(eventID, func(t *testing.T) {
			evt, ok := events[eventID]
			if !ok {
				t.Fatalf("fixture has no event %s", eventID)
			}
			got, accepted := messageFrom(evt, id.UserID(selfMXID))
			if !accepted {
				t.Fatalf("%s was refused, want accepted", eventID)
			}
			assertMessage(t, got, w)
		})
	}
}

// assertMessage pins every field of an accepted message, including the
// timestamp: the router's staleness rule drops a weekend backlog, and a
// timestamp nobody asserted is a backlog that fires.
func assertMessage(t *testing.T, got Message, w want) {
	t.Helper()
	if got.Adapter != "matrix" {
		t.Errorf("adapter = %q, want matrix", got.Adapter)
	}
	if got.Sender != w.sender {
		t.Errorf("sender = %q, want %q", got.Sender, w.sender)
	}
	if got.Room != w.room {
		t.Errorf("room = %q, want %q", got.Room, w.room)
	}
	if got.Event != w.event {
		t.Errorf("event = %q, want %q (the reply target)", got.Event, w.event)
	}
	if got.Text != w.text {
		t.Errorf("text = %q, want %q", got.Text, w.text)
	}
	if got.At != time.UnixMilli(w.at) {
		t.Errorf("at = %s, want %s", got.At, time.UnixMilli(w.at))
	}
}

// fixtureEvents unmarshals the payload and indexes every timeline event by ID.
// ParseRaw is mandatory here: event.Content.AsMessage reads the already-parsed
// field and never touches the raw JSON, so an unparsed fixture would make
// every message look empty and every case pass for the wrong reason.
func fixtureEvents(t *testing.T) map[string]*event.Event {
	t.Helper()
	var resp mautrix.RespSync
	if err := json.Unmarshal([]byte(syncFixture), &resp); err != nil {
		t.Fatalf("unmarshalling the sync fixture: %v", err)
	}
	events := map[string]*event.Event{}
	for _, room := range resp.Rooms.Join {
		for _, evt := range room.Timeline.Events {
			if err := evt.Content.ParseRaw(evt.Type); err != nil {
				t.Fatalf("parsing %s content: %v", evt.ID, err)
			}
			events[evt.ID.String()] = evt
		}
	}
	if len(events) == 0 {
		t.Fatal("fixture produced no events")
	}
	return events
}
