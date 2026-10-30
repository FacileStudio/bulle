package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/FacileStudio/bulle/internal/chat"
)

func startChatIPC(ctx context.Context, m chatMatrix, adapter chat.Adapter, rooms *roomRegistry) {
	send := func(c context.Context, req chat.IPCRequest) error {
		return handleIPCSend(c, m, adapter, req)
	}
	trigger := func(c context.Context, room, prompt string) (string, error) {
		return rooms.handleIPCTrigger(c, m, adapter, room, prompt)
	}
	go runIPCServer(ctx, send, trigger)
}

func handleIPCSend(ctx context.Context, m chatMatrix, adapter chat.Adapter, req chat.IPCRequest) error {
	target := resolveTargetRoom(req.Room, m.rooms)
	if target == "" {
		return errors.New("no room specified and no default room configured")
	}
	return adapter.Send(ctx, chat.Identity{Adapter: "matrix", Room: target}, req.Text)
}

func (rooms *roomRegistry) handleIPCTrigger(ctx context.Context, m chatMatrix, adapter chat.Adapter, room, prompt string) (string, error) {
	target := resolveTargetRoom(room, m.rooms)
	if target == "" {
		return "", errors.New("no room specified and no default room configured")
	}
	key := target
	if !strings.HasPrefix(key, "matrix:") {
		key = "matrix:" + key
	}
	r, err := rooms.get(key)
	if err != nil {
		return "", err
	}
	reply, err := r.answer(ctx, prompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(reply) != "" && adapter != nil {
		if sendErr := adapter.Send(ctx, chat.Identity{Adapter: "matrix", Room: target}, reply); sendErr != nil {
			return reply, fmt.Errorf("matrix send: %w", sendErr)
		}
	}
	return reply, nil
}

func resolveTargetRoom(room string, configured []string) string {
	if room != "" {
		return room
	}
	if len(configured) > 0 {
		return configured[0]
	}
	return ""
}

func runIPCServer(ctx context.Context, send chat.IPCSend, trigger chat.IPCTrigger) {
	if err := chat.ServeIPC(ctx, "", send, trigger); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "bulle chat ipc: %v\n", err)
	}
}
