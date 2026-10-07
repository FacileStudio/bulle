package cmd

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
)

type chatRoom struct {
	mu        sync.Mutex
	session   *engine.Session
	compactor *compaction.Engine
	log       *sessions.SessionLog
	cleanup   func()
}

type roomRegistry struct {
	mu     sync.Mutex
	config settings.Config
	rooms  map[string]*chatRoom
}

func newRoomRegistry(config settings.Config) *roomRegistry {
	return &roomRegistry{
		config: config,
		rooms:  make(map[string]*chatRoom),
	}
}

func (r *roomRegistry) get(key string) (*chatRoom, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if room, ok := r.rooms[key]; ok {
		return room, nil
	}
	room, err := newChatRoom(r.config)
	if err != nil {
		return nil, err
	}
	r.rooms[key] = room
	return room, nil
}

// Close releases resources held by active room sessions.
func (r *roomRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, room := range r.rooms {
		if room.cleanup != nil {
			room.cleanup()
		}
	}
	return nil
}

func newChatRoom(config settings.Config) (*chatRoom, error) {
	a, cleanup, err := agent.BuildHeadlessAgent(config, nil)
	if err != nil {
		return nil, err
	}
	compactor := agent.ChatCompactor(config, a)
	conv := engine.NewConversation()
	session := engine.NewSession(a, conv)
	log := sessions.OpenSession(config.Backend, config.Model, config.Root)
	return &chatRoom{
		session:   session,
		compactor: compactor,
		log:       log,
		cleanup:   cleanup,
	}, nil
}

func (r *chatRoom) answer(ctx context.Context, text string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compactIfNeeded(ctx)
	if r.log != nil {
		r.log.Line(sessions.FromReader, text)
	}
	events, err := r.session.Submit(ctx, text)
	if err != nil {
		return "", err
	}
	reply, err := r.drain(ctx, events)
	if err != nil && engine.DetectOverflow(err) {
		return r.retryOverflow(ctx)
	}
	if err != nil {
		return "", err
	}
	if r.log != nil {
		r.log.Line(sessions.FromModel, reply)
	}
	return reply, nil
}

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
