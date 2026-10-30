package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/bulle/internal/memory"
	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/settings"
)

type chatRoom struct {
	mu        sync.Mutex
	pmu       sync.Mutex
	pending   []string
	session   *engine.Session
	compactor *compaction.Engine
	log       *sessions.SessionLog
	cleanup   func()
	path      string
	day       string
	loc       *time.Location
	now       func() time.Time
	summarize func(context.Context, []nacelle.Message) string
	memory    *memory.Store
}

type roomRegistry struct {
	mu     sync.Mutex
	config settings.Config
	rooms  map[string]*chatRoom
	asker  agent.Asker
	nudges *nudgeLog
	memory *memory.Store
}

func newRoomRegistry(config settings.Config) *roomRegistry {
	if autoApprove(config) {
		fmt.Fprintln(os.Stderr, "bulle chat: auto_approve is on, every tool call runs without asking and is logged here")
	}
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
	room, err := newChatRoom(r.config, key, r.asker, r.memory)
	if err != nil {
		return nil, err
	}
	r.rooms[key] = room
	return room, nil
}

func (r *roomRegistry) useAsker(asker agent.Asker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asker = asker
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

func newChatRoom(config settings.Config, key string, asker agent.Asker, mem *memory.Store) (*chatRoom, error) {
	config, tools := withMemory(config, mem)
	a, cleanup, err := agent.BuildChatAgent(config, nil, agent.ChatApprove(key, asker, autoApprove(config)), tools...)
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
		memory:    mem,
	}
	room.summarize = room.compactSummary
	return room, nil
}

func (r *chatRoom) answer(ctx context.Context, text string) (string, error) {
	if err := r.lockCtx(ctx); err != nil {
		return "", err
	}
	defer r.mu.Unlock()
	r.rollDay(ctx)
	r.flushPending()
	r.compactIfNeeded(ctx)
	reply, err := r.turn(ctx, text)
	r.flushPending()
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

func autoApprove(config settings.Config) bool {
	flag := config.Chat.AutoApprove
	return flag != nil && *flag
}
