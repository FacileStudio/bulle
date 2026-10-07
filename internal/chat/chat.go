// Package chat turns one inbound platform message into one bulle run, and the
// answer back into the room it came from.
//
// An adapter is thin on purpose: it owns a platform connection and nothing
// else. Authorization, session keying and staleness live here, in one place,
// so a second platform cannot arrive with a second opinion about who is
// allowed to make this machine run a tool.
package chat

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Identity names one conversation: which adapter, which room, which human. It
// is where a message came from and where its answer goes, so an adapter never
// has to be told twice.
//
// Event is the platform's own id for the message being answered, and an empty
// one means the reply stands alone rather than attaching to a question. It is
// not part of Key: a conversation is a room, and the event a reply hangs off
// is different on every turn.
type Identity struct {
	Adapter string `json:"adapter"`
	Room    string `json:"room"`
	Sender  string `json:"sender"`
	Event   string `json:"event,omitempty"`
}

// Key is the session key of this conversation, and it is the room rather than
// the project deliberately: --continue resumes the newest session for a
// project, so two chats in one repository would interleave into one
// conversation. The sender is left out because two people in one room are
// talking to the same session.
func (i Identity) Key() string {
	return i.Adapter + ":" + i.Room
}

// Message is one inbound text with the identity it arrived under.
type Message struct {
	Identity
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Adapter is one platform connection. Name identifies it and is the string
// every Identity it produces carries. Receive blocks until the context is
// cancelled, handing each inbound message to handle in arrival order. Send
// delivers one reply, in plaintext where the platform has encryption.
type Adapter interface {
	Name() string
	Receive(ctx context.Context, handle func(Message)) error
	Send(ctx context.Context, to Identity, text string) error
}

// Typer is an optional interface an adapter can implement to indicate typing.
type Typer interface {
	Typing(ctx context.Context, to Identity, typing bool) error
}

// Responder answers one message. It is a function rather than an interface
// because a daemon has exactly one of them: the run itself.
type Responder func(ctx context.Context, m Message) (string, error)

// Run drives one adapter until its context is cancelled. Every message the
// router accepts becomes one answer, sent back through the adapter that
// received it. Messages in one conversation are answered in turn; different
// conversations run at the same time.
func Run(ctx context.Context, a Adapter, respond Responder, router Router) error {
	inTurn := newSerial()
	var wg sync.WaitGroup
	err := a.Receive(ctx, func(m Message) {
		if refused := router.Check(m, time.Now()); refused != nil {
			report(a.Name(), refused)
			return
		}
		gate, release := inTurn.wait(m.Key())
		wg.Go(func() {
			if gate != nil {
				<-gate
			}
			defer release()
			answer(ctx, a, respond, m)
		})
	})
	wg.Wait()
	return err
}

// answer runs one message and replies with what came back. A failed run still
// gets a reply: a human who typed into a room and got silence cannot tell a
// refusal from a crash from a bot that is simply down.
func answer(ctx context.Context, a Adapter, respond Responder, m Message) {
	stop := startTyping(ctx, a, m.Identity)
	defer stop()
	text, err := respond(ctx, m)
	stop()
	if err != nil {
		report(a.Name(), err)
		text = "bulle could not run that: " + oneLine(err)
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	if err := a.Send(ctx, m.Identity, text); err != nil {
		report(a.Name(), err)
	}
}

func startTyping(ctx context.Context, a Adapter, to Identity) func() {
	typer, ok := a.(Typer)
	if !ok {
		return func() {}
	}
	tryTyping(ctx, typer, to, true)
	done := make(chan struct{})
	var once sync.Once
	go runTypingLoop(ctx, typer, to, done)
	return func() {
		once.Do(func() {
			close(done)
			stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			tryTyping(stopCtx, typer, to, false)
		})
	}
}

func runTypingLoop(ctx context.Context, typer Typer, to Identity, done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			tryTyping(ctx, typer, to, true)
		}
	}
}

func tryTyping(ctx context.Context, typer Typer, to Identity, typing bool) {
	if err := typer.Typing(ctx, to, typing); err != nil {
		report(to.Adapter, err)
	}
}

// oneLine flattens an error into something a chat message can carry, since a
// platform renders its own line breaks and a multi-line error arrives as a
// wall of text from an anonymous process.
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

func report(name string, err error) {
	fmt.Fprintf(os.Stderr, "bulle chat: %s: %s\n", name, oneLine(err))
}
