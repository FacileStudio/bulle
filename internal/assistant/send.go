package assistant

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

const (
	sendAttempts = 3
	sendGap      = 30 * time.Second
)

// DefaultSend sends through the chat daemon socket and turns a refusal into
// an error.
func DefaultSend(ctx context.Context, req chat.IPCRequest) error {
	resp, err := chat.SendIPC(ctx, req)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New("chat daemon refused: " + resp.Error)
	}
	return nil
}

func deliver(ctx context.Context, d Deps, req chat.IPCRequest) string {
	var err error
	for i := 0; i < sendAttempts; i++ {
		if err = d.Send(ctx, req); err == nil {
			return DeliveryOK
		}
		if i < sendAttempts-1 {
			d.Sleep(sendGap)
		}
	}
	fmt.Fprintf(d.Log, "assistant: delivery failed after %d attempts: %v\n", sendAttempts, err)
	if ferr := writeFallback(d, req.Text); ferr != nil {
		fmt.Fprintf(d.Log, "assistant: fallback write failed: %v\n", ferr)
	}
	return DeliveryFailed
}

func writeFallback(d Deps, text string) error {
	if err := os.MkdirAll(d.LogDir, 0o700); err != nil {
		return err
	}
	now := d.Now()
	path := filepath.Join(d.LogDir, "assistant-fallback-"+now.Format("2006-01-02")+".md")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n=== %s ===\n%s\n", now.Format(time.RFC3339), text)
	return err
}
