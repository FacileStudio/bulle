package cmd

import (
	"context"
	"path/filepath"

	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/memory"
	"github.com/FacileStudio/bulle/internal/settings"
)

func newChatServices(adapter *chat.Matrix, rooms *roomRegistry, reactions *chat.ReactionRouter) (*chatServices, error) {
	mem, err := memory.Default()
	if err != nil {
		return nil, err
	}
	ledger, err := assistant.NewLedger("")
	if err != nil {
		return nil, err
	}
	cfg, err := assistantSettings()
	if err != nil {
		return nil, err
	}
	home, err := settings.HomeDir()
	if err != nil {
		return nil, err
	}
	ap := newApprovals(adapter, reactions)
	rooms.useAsker(ap)
	rooms.nudges = newNudgeLog(reactions)
	rooms.memory = mem
	return &chatServices{
		rooms: rooms, adapter: adapter, approvals: ap, ledger: ledger,
		memory: mem, assistant: cfg, stateDir: filepath.Join(home, "state"),
	}, nil
}

func startAntenneWatch(ctx context.Context) {
	deps, err := assistantDeps()
	if err != nil {
		return
	}
	go assistant.WatchAntenne(ctx, func() { _, _ = assistant.Watch(ctx, deps) })
}
