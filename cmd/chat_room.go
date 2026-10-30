package cmd

import (
	"context"
	"sync"
	"time"

	"github.com/FacileStudio/nacelle"

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
	path      string
	day       string
	loc       *time.Location
	now       func() time.Time
	summarize func(context.Context, []nacelle.Message) string
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
	room, err := newChatRoom(r.config, key)
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

func newChatRoom(config settings.Config, key string) (*chatRoom, error) {
	a, cleanup, err := agent.BuildHeadlessAgent(config, nil)
	if err != nil {
		return nil, err
	}
	path, _ := roomStorePath(key)
	day, msgs := loadRoom(path)
	room := &chatRoom{
		session:   engine.NewSession(a, engine.NewConversation(msgs...)),
		compactor: agent.ChatCompactor(config, a),
		log:       sessions.OpenSession(config.Backend, config.Model, config.Root),
		cleanup:   cleanup,
		path:      path,
		day:       day,
		loc:       assistantLocation(config),
		now:       time.Now,
	}
	room.summarize = room.compactSummary
	return room, nil
}

func (r *chatRoom) answer(ctx context.Context, text string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rollDay(ctx)
	r.compactIfNeeded(ctx)
	reply, err := r.turn(ctx, text)
	r.persist()
	return reply, err
}

func (r *chatRoom) turn(ctx context.Context, text string) (string, error) {
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
