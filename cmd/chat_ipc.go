package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/FacileStudio/bulle/internal/chat"
)

func startChatIPC(ctx context.Context, m chatMatrix, adapter chat.Adapter, rooms *roomRegistry) {
	send := func(c context.Context, room, text string) error {
		return handleIPCSend(c, m, adapter, room, text)
	}
	trigger := func(c context.Context, room, prompt string) (string, error) {
		return handleIPCTrigger(c, m, rooms, room, prompt)
	}
	go runIPCServer(ctx, send, trigger)
}

func handleIPCSend(ctx context.Context, m chatMatrix, adapter chat.Adapter, room, text string) error {
	target := resolveTargetRoom(room, m.rooms)
	if target == "" {
		return errors.New("no room specified and no default room configured")
	}
	return adapter.Send(ctx, chat.Identity{Adapter: "matrix", Room: target}, text)
}

func handleIPCTrigger(ctx context.Context, m chatMatrix, rooms *roomRegistry, room, prompt string) (string, error) {
	target := resolveTargetRoom(room, m.rooms)
	r, err := rooms.get(target)
	if err != nil {
		return "", err
	}
	return r.answer(ctx, prompt)
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

func runIPCServer(ctx context.Context, send func(context.Context, string, string) error, trigger func(context.Context, string, string) (string, error)) {
	if err := chat.ServeIPC(ctx, "", send, trigger); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "bulle chat ipc: %v\n", err)
	}
}
