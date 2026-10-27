package chat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitForSocket(t *testing.T, sock string) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for socket %s", sock)
}

func TestIPCSend(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var gotRoom, gotText string
	send := func(_ context.Context, room, text string) error {
		gotRoom = room
		gotText = text
		return nil
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(ctx, sock, send, nil)
	}()
	waitForSocket(t, sock)
	req := IPCRequest{Action: "send", Room: "!room:example.org", Text: "hello"}
	resp, err := SendIPCTo(context.Background(), sock, req)
	if err != nil {
		t.Fatalf("SendIPCTo failed: %v", err)
	}
	if !resp.OK || resp.Error != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if gotRoom != "!room:example.org" || gotText != "hello" {
		t.Fatalf("got room %q text %q", gotRoom, gotText)
	}
	cancel()
	<-errCh
}

func TestIPCTrigger(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	trigger := func(_ context.Context, room, prompt string) (string, error) {
		if room != "!room:example.org" {
			return "", fmt.Errorf("wrong room: %s", room)
		}
		return "reply to " + prompt, nil
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(ctx, sock, nil, trigger)
	}()
	waitForSocket(t, sock)
	req := IPCRequest{Action: "trigger", Room: "!room:example.org", Prompt: "ping"}
	resp, err := SendIPCTo(context.Background(), sock, req)
	if err != nil {
		t.Fatalf("SendIPCTo failed: %v", err)
	}
	if !resp.OK || resp.Reply != "reply to ping" {
		t.Fatalf("got response %+v", resp)
	}
	cancel()
	<-errCh
}

func TestIPCErrors(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	send := func(_ context.Context, _, _ string) error {
		return errors.New("send failed")
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(ctx, sock, send, nil)
	}()
	waitForSocket(t, sock)
	r1, _ := SendIPCTo(context.Background(), sock, IPCRequest{Action: "send", Text: "hi"})
	if r1.OK || r1.Error != "send failed" {
		t.Fatalf("expected send error, got %+v", r1)
	}
	r2, _ := SendIPCTo(context.Background(), sock, IPCRequest{Action: "trigger", Prompt: "hi"})
	if r2.OK || r2.Error != "trigger handler not configured" {
		t.Fatalf("expected trigger unconfigured error, got %+v", r2)
	}
	r3, _ := SendIPCTo(context.Background(), sock, IPCRequest{Action: "unknown"})
	if r3.OK || !strings.Contains(r3.Error, "unknown action") {
		t.Fatalf("expected unknown action error, got %+v", r3)
	}
	cancel()
	<-errCh
}

func TestIPCSocketCleanup(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(ctx, sock, nil, nil)
	}()
	waitForSocket(t, sock)
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial socket failed: %v", err)
	}
	defer conn.Close()
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeIPC did not terminate on cancel")
	}
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected conn to be closed on cancel")
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("socket still exists: %v", err)
	}
}

func TestIPCStaleSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("failed to create fake socket: %v", err)
	}
	ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(ctx, sock, nil, nil)
	}()
	waitForSocket(t, sock)
	resp, err := SendIPCTo(context.Background(), sock, IPCRequest{Action: "send"})
	if err != nil || resp.OK {
		t.Fatalf("expected send handler error, got resp=%+v err=%v", resp, err)
	}
	cancel()
	<-errCh
}

func TestIPCActiveSocketConflict(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIPC(ctx, sock, nil, nil)
	}()
	waitForSocket(t, sock)
	err := ServeIPC(ctx, sock, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected already in use error, got: %v", err)
	}
	cancel()
	<-errCh
}

func TestSocketPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		path, err := SocketPath()
		if err != nil {
			t.Fatalf("SocketPath failed: %v", err)
		}
		if !strings.HasSuffix(path, "chat.sock") {
			t.Fatalf("expected path ending in chat.sock, got %q", path)
		}
	})
	t.Run("env", func(t *testing.T) {
		custom := filepath.Join(t.TempDir(), "custom.sock")
		t.Setenv("BULLE_CHAT_SOCK", custom)
		got, err := SocketPath()
		if err != nil {
			t.Fatalf("SocketPath failed: %v", err)
		}
		if got != custom {
			t.Fatalf("expected %q, got %q", custom, got)
		}
	})
}
