package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

const ipcTriggerTimeout = 10 * time.Minute

func startChatIPC(ctx context.Context, m chatMatrix, adapter chat.Adapter, rooms *roomRegistry) {
	send := func(c context.Context, req chat.IPCRequest) error {
		return rooms.handleIPCSend(c, m, adapter, req)
	}
	trigger := func(c context.Context, room, prompt string) (string, error) {
		return rooms.handleIPCTrigger(c, m, adapter, room, prompt)
	}
	go runIPCServer(ctx, send, trigger)
}

func (rooms *roomRegistry) handleIPCSend(ctx context.Context, m chatMatrix, adapter chat.Adapter, req chat.IPCRequest) error {
	target := resolveTargetRoom(req.Room, m.rooms)
	if target == "" {
		return errors.New("no room specified and no default room configured")
	}
	notify := req.Notify == nil || *req.Notify
	id, err := chat.Deliver(ctx, adapter, chat.Identity{Adapter: "matrix", Room: target}, req.Text, notify)
	if err != nil {
		return err
	}
	if rooms.nudges != nil {
		rooms.nudges.note(target, id, req.Items)
	}
	if req.Record {
		rooms.recordNudge(target, req)
	}
	return nil
}

func (rooms *roomRegistry) recordNudge(target string, req chat.IPCRequest) {
	r, err := rooms.get(matrixRoomKey(target))
	if err != nil {
		fmt.Fprintf(os.Stderr, "bulle chat: record nudge: %v\n", err)
		return
	}
	r.record(recordedText(req))
}

func recordedText(req chat.IPCRequest) string {
	if req.RecordText != "" {
		return req.RecordText
	}
	return req.Text
}

func matrixRoomKey(target string) string {
	if strings.HasPrefix(target, "matrix:") {
		return target
	}
	return "matrix:" + target
}

func (rooms *roomRegistry) handleIPCTrigger(ctx context.Context, m chatMatrix, adapter chat.Adapter, room, prompt string) (string, error) {
	target := resolveTargetRoom(room, m.rooms)
	if target == "" {
		return "", errors.New("no room specified and no default room configured")
	}
	r, err := rooms.get(matrixRoomKey(target))
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, ipcTriggerTimeout)
	defer cancel()
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
