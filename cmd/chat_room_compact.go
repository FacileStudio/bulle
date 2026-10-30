package cmd

import (
	"context"
	"errors"
	"strings"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/bulle/internal/sessions"
)

func (r *chatRoom) compactIfNeeded(ctx context.Context) {
	if r.compactor == nil {
		return
	}
	conv := r.session.Conversation()
	msgs := conv.Messages()
	if len(msgs) == 0 {
		return
	}
	size := compaction.EstTokens(compaction.Bytes(msgs))
	trigger := r.compactor.Policy.Trigger()
	if trigger <= 0 || size < trigger {
		return
	}
	outcome := r.compactor.RunPassWithSize(ctx, msgs, size, false)
	if outcome.Err == nil && outcome.Installs() {
		newConv, _ := outcome.Apply(msgs)
		conv.Replace(newConv)
	}
}

func (r *chatRoom) retryOverflow(ctx context.Context) (string, error) {
	if r.compactor == nil {
		return "", errors.New("chat: context overflow with no compactor")
	}
	conv := r.session.Conversation()
	msgs := conv.Messages()
	outcome := r.compactor.RunPass(ctx, msgs, true)
	if outcome.Err != nil {
		return "", outcome.Err
	}
	if !outcome.Installs() {
		return "", errors.New("chat: context overflow compaction failed")
	}
	newConv, _ := outcome.Apply(msgs)
	conv.Replace(newConv)
	events, err := r.session.Submit(ctx, "")
	if err != nil {
		return "", err
	}
	reply, err := r.drain(ctx, events)
	if err != nil {
		return "", err
	}
	if r.log != nil {
		r.log.Line(sessions.FromModel, reply)
	}
	return reply, nil
}

func (r *chatRoom) drain(ctx context.Context, events <-chan engine.Event) (string, error) {
	var out strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case event, ok := <-events:
			if !ok {
				return strings.TrimSpace(out.String()), nil
			}
			switch event.Kind {
			case engine.EventError:
				return "", event.Err
			case engine.EventTextDelta:
				out.WriteString(event.Text)
			}
		}
	}
}
