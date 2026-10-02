package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// joinRecorder is a stand-in homeserver that records which rooms were joined
// and answers the join the way a real one does.
type joinRecorder struct {
	mu     sync.Mutex
	joined []string
}

func (j *joinRecorder) handle(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/join") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	j.mu.Lock()
	j.joined = append(j.joined, r.URL.Path)
	j.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"room_id":"!r:example.org"}`))
}

func (j *joinRecorder) rooms() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.joined...)
}

// joinProbe is the adapter minus the crypto machine: a client pointed at a fake
// homeserver, and the same invite handler the daemon registers. No device and
// no database are involved, which is the point: joining a room is an ordinary
// request and must not depend on either.
type joinProbe struct {
	matrix *Matrix
	server *joinRecorder
}

func newJoinProbe(t *testing.T, self id.UserID) *joinProbe {
	t.Helper()
	rec := &joinRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handle))
	t.Cleanup(srv.Close)
	client, err := mautrix.NewClient(srv.URL, self, "syt_test")
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}
	m := &Matrix{client: client, self: self}
	if err := m.watchInvites(); err != nil {
		t.Fatalf("registering the invite handler: %v", err)
	}
	return &joinProbe{matrix: m, server: rec}
}

// deliver sends the membership event a homeserver produces for an invite, in
// the raw form the syncer sees and parses before dispatch. It is not built with
// a pre-parsed Content: the handler reads that parsed field, so a fixture that
// filled it in would pass even if the real path never parsed anything.
func (p *joinProbe) deliver(t *testing.T, stateKey id.UserID, membership event.Membership) {
	t.Helper()
	key := stateKey.String()
	raw, err := json.Marshal(map[string]string{"membership": string(membership)})
	if err != nil {
		t.Fatalf("building the membership content: %v", err)
	}
	evt := &event.Event{
		Type:     event.StateMember,
		RoomID:   "!r:example.org",
		StateKey: &key,
		Sender:   "@alice:example.org",
		Content:  event.Content{VeryRaw: raw},
	}
	if err := evt.Content.ParseRaw(evt.Type); err != nil {
		t.Fatalf("parsing the membership content: %v", err)
	}
	syncer, ok := p.matrix.client.Syncer.(mautrix.DispatchableSyncer)
	if !ok {
		t.Fatal("the client syncer cannot dispatch")
	}
	syncer.Dispatch(context.Background(), evt)
}

// TestAnInviteMakesTheBotJoin is the whole answer to how the bot gets into a
// room: nobody runs a join command. The invite is the trigger. Matrix delivers
// an invite as stripped state rather than a room timeline event, so this pins
// that the handler still sees it, and that the room id is passed through with
// the sigils a room id carries escaped into a valid path.
func TestAnInviteMakesTheBotJoin(t *testing.T) {
	self := id.UserID("@bulle:example.org")
	p := newJoinProbe(t, self)
	p.deliver(t, self, event.MembershipInvite)

	joined := p.server.rooms()
	if len(joined) != 1 {
		t.Fatalf("join requests = %d (%v), want exactly 1", len(joined), joined)
	}
	want := "/_matrix/client/v3/rooms/!r:example.org/join"
	if joined[0] != want {
		t.Errorf("joined %q, want %q", joined[0], want)
	}
}

// TestOnlyTheBotsOwnInviteIsJoined keeps the trigger narrow. Invite state also
// carries membership events for other people, so a handler that joined on any
// invite would pull the bot into rooms nobody asked it into.
func TestOnlyTheBotsOwnInviteIsJoined(t *testing.T) {
	self := id.UserID("@bulle:example.org")
	p := newJoinProbe(t, self)

	p.deliver(t, "@someone-else:example.org", event.MembershipInvite)
	if joined := p.server.rooms(); len(joined) != 0 {
		t.Fatalf("joined %v for another user's invite, want no join", joined)
	}
	p.deliver(t, self, event.MembershipJoin)
	if joined := p.server.rooms(); len(joined) != 0 {
		t.Fatalf("joined %v for a membership that is not an invite, want no join", joined)
	}
}
