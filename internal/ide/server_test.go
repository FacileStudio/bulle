package ide

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestHelloOpensEveryAttachment(t *testing.T) {
	srv := startTestServer(t, Options{Root: "/repo", Model: "claude", Version: "v0.76.0", Session: "/tmp/s.jsonl"})
	sock, err := net.Dial("unix", srv.path)
	if err != nil {
		t.Fatalf("dialling the IDE socket: %v", err)
	}
	defer func() { _ = sock.Close() }()

	ev := readEvent(t, NewDecoder(sock))
	wantString(t, ev, "t", "hello")
	wantString(t, ev, "root", "/repo")
	wantString(t, ev, "session", "/tmp/s.jsonl")
	wantString(t, ev, "model", "claude")
	wantString(t, ev, "version", "v0.76.0")
	wantNumber(t, ev, "v", Protocol)
	wantNumber(t, ev, "pid", float64(os.Getpid()))
}

func TestUnsupportedProtocolVersionIsRefusedAndClosed(t *testing.T) {
	srv := startTestServer(t, Options{})
	sock, dec := attach(t, srv)

	write(t, sock, `{"v":99,"t":"hello"}`)
	ev := readEvent(t, dec)
	wantString(t, ev, "t", "error")
	wantString(t, ev, "reason", "unsupported protocol version")

	var missed map[string]any
	if err := dec.Decode(&missed); err == nil {
		t.Error("the connection stayed open after refusing a protocol version")
	}
}

func TestUnknownTypesAndLateAnswersAreIgnored(t *testing.T) {
	srv := startTestServer(t, Options{})
	sock, dec := attach(t, srv)

	write(t, sock, `{"v":1,"t":"approve","id":"nobody-is-waiting","allow":true}`)
	write(t, sock, `{"v":1,"t":"something-new","field":1}`)
	srv.Publish(Turn(1))

	ev := readEvent(t, dec)
	wantString(t, ev, "t", "turn")
	wantNumber(t, ev, "n", 1)
}

func TestPublishReachesEveryAttachedEditor(t *testing.T) {
	srv := startTestServer(t, Options{})
	_, first := attach(t, srv)
	_, second := attach(t, srv)

	srv.Publish(ToolDone("edit_file-1", "edit_file", true, "a.go"))
	for _, dec := range []*Decoder{first, second} {
		ev := readEvent(t, dec)
		wantString(t, ev, "t", "tool")
		wantString(t, ev, "status", "done")
	}
}

// TestApproveIsUnansweredWithNoEditorAttached pins the difference between "no
// editor to ask" and "the editor said no". A session that opens a socket nobody
// has attached to must leave the decision where it was — a caller with its own
// approval surface, which is bulle's terminal prompt, keeps it — rather than
// reporting a refusal it never asked for.
func TestApproveIsUnansweredWithNoEditorAttached(t *testing.T) {
	srv := startTestServer(t, Options{})

	got := srv.Approve(context.Background(), "call-1", "run_command", `{"command":"ls"}`)
	if got != Unanswered {
		t.Errorf("Approve = %v with nobody attached, want Unanswered", got)
	}
}

func TestApproveCarriesTheCallAndTakesTheAnswer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		allow bool
		want  Answer
	}{{"yes", true, Allowed}, {"no", false, Refused}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := answerOnce(t, tc.allow); got != tc.want {
				t.Errorf("Approve = %v, want %v", got, tc.want)
			}
		})
	}
}

// answerOnce attaches one editor, answers a pending call with allow, and
// returns what the session made of that answer. The surface goes back down
// here rather than waiting for the test's own cleanup.
func answerOnce(t *testing.T, allow bool) Answer {
	t.Helper()
	srv := startTestServer(t, Options{})
	defer func() { _ = srv.Close() }()
	sock, dec := attach(t, srv)

	answers := make(chan Answer, 1)
	go func() {
		answers <- srv.Approve(context.Background(), "call-1", "run_command", `{"command":"ls"}`)
	}()

	ev := readEvent(t, dec)
	wantString(t, ev, "t", "approval")
	wantString(t, ev, "id", "call-1")
	wantString(t, ev, "tool", "run_command")
	wantString(t, ev, "input", `{"command":"ls"}`)

	write(t, sock, fmt.Sprintf(`{"v":1,"t":"approve","id":"call-1","allow":%t}`, allow))
	select {
	case got := <-answers:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("the approval never came back")
		return Unanswered
	}
}

// TestApproveFailsClosedWhenTheEditorGoesAway pins that an editor which
// vanishes mid-wait is a refusal and not Unanswered: the call was asked, so the
// caller's own surface must not be handed it afterwards, or one call would be
// answered by two different humans.
func TestApproveFailsClosedWhenTheEditorGoesAway(t *testing.T) {
	srv := startTestServer(t, Options{})
	sock, dec := attach(t, srv)

	answers := make(chan Answer, 1)
	go func() { answers <- srv.Approve(context.Background(), "call-1", "run_command", "{}") }()
	readEvent(t, dec)
	sock.Close()

	select {
	case got := <-answers:
		if got != Refused {
			t.Errorf("Approve = %v after a lost connection, want Refused", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the approval never came back after the editor went away")
	}
}

// write sends one line to the session, the way an editor does.
func write(t *testing.T, sock net.Conn, line string) {
	t.Helper()
	if _, err := sock.Write([]byte(line + "\n")); err != nil {
		t.Fatalf("writing %s: %v", line, err)
	}
}
