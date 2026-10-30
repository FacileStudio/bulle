package chat

import (
	"context"
	"sync"
	"time"
)

type daemon struct {
	a       Adapter
	respond Responder
	router  Router
	cmds    *Commands
	inTurn  *serial
	turns   *turns
	wg      sync.WaitGroup
}

// RunWith is Run with the in-code commands, reactions and audio of cmds. A nil
// cmds behaves exactly like Run. /stop is handled the moment it arrives, past
// the queue, so it can cancel the turn that the queue is waiting on.
func RunWith(ctx context.Context, a Adapter, respond Responder, router Router, cmds *Commands) error {
	d := &daemon{a: a, respond: respond, router: router, cmds: cmds, inTurn: newSerial(), turns: newTurns()}
	err := a.Receive(ctx, func(m Message) { d.accept(ctx, m) })
	d.wg.Wait()
	return err
}

func (d *daemon) accept(ctx context.Context, m Message) {
	if refused := d.router.Check(m, time.Now()); refused != nil {
		report(d.a.Name(), refused)
		return
	}
	if d.cmds != nil && d.cmds.Intercept != nil && m.Is(KindText) && d.cmds.Intercept(m) {
		return
	}
	name, _, _ := d.cmds.match(m)
	switch {
	case m.Is(KindReaction):
		d.wg.Go(func() { d.cmds.react(ctx, m) })
	case m.Is(KindAudio) && (d.cmds == nil || d.cmds.Audio == nil):
		return
	case name == "stop":
		d.wg.Go(func() { d.stop(ctx, m) })
	default:
		d.queue(ctx, m)
	}
}

func (d *daemon) queue(ctx context.Context, m Message) {
	markSeen(ctx, d.a, m.Identity)
	epoch := d.turns.epoch(m.Key())
	gate, release := d.inTurn.wait(m.Key())
	d.wg.Go(func() {
		if gate != nil {
			<-gate
		}
		defer release()
		if d.turns.epoch(m.Key()) != epoch {
			return
		}
		answer(ctx, d.a, d.responder(), m)
	})
}

func (d *daemon) stop(ctx context.Context, m Message) {
	text := "nothing is running."
	if d.turns.stop(m.Key()) {
		text = "stopped."
	}
	if _, h, args := d.cmds.match(m); h != nil {
		text = h(ctx, m, args)
	}
	if text == "" {
		return
	}
	if err := d.a.Send(ctx, m.Identity, text); err != nil {
		report(d.a.Name(), err)
	}
}

func (d *daemon) responder() Responder {
	return func(ctx context.Context, m Message) (string, error) {
		turn, done := d.turns.begin(ctx, m.Key())
		defer done()
		text, err := d.turn(turn, m)
		if turn.Err() != nil && ctx.Err() == nil {
			return "stopped.", nil
		}
		return text, err
	}
}

func (d *daemon) turn(ctx context.Context, m Message) (string, error) {
	if m.Is(KindAudio) {
		return d.cmds.Audio(ctx, m, ""), nil
	}
	if _, h, args := d.cmds.match(m); h != nil {
		return h(ctx, m, args), nil
	}
	return d.respond(ctx, m)
}
