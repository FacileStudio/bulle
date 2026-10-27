package cmd

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

func TestChatHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.77.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"chat", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on chat help, got: %v", err)
	}
	for _, sub := range []string{"channels", "install", "send", "trigger", "uninstall", "verify"} {
		if !bytes.Contains(buf.Bytes(), []byte(sub)) {
			t.Errorf("expected %s subcommand in chat help, got: %s", sub, buf.String())
		}
	}
}

func TestChatSendHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.77.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"chat", "send", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on chat send help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("--room")) {
		t.Errorf("expected --room flag in chat send help, got: %s", buf.String())
	}
}

func TestChatTriggerHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.77.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"chat", "trigger", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on chat trigger help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("--room")) {
		t.Errorf("expected --room flag in chat trigger help, got: %s", buf.String())
	}
}

func TestChatVerifyHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.77.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"chat", "verify", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on chat verify help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("--recovery-key")) {
		t.Errorf("expected --recovery-key flag in chat verify help, got: %s", buf.String())
	}
}

func TestChatSendCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sock := filepath.Join(home, ".bulle", "chat.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var gotRoom, gotText string
	send := func(_ context.Context, room, text string) error {
		gotRoom = room
		gotText = text
		return nil
	}
	go func() {
		chat.ServeIPC(ctx, sock, send, nil)
	}()
	waitForSock(t, sock)
	cmd := newRootCmd("v0.77.0")
	cmd.SetArgs([]string{"chat", "send", "--room", "!r:matrix.org", "hello world"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if gotRoom != "!r:matrix.org" || gotText != "hello world" {
		t.Fatalf("got room %q text %q", gotRoom, gotText)
	}
}

func TestChatTriggerCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sock := filepath.Join(home, ".bulle", "chat.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	trigger := func(_ context.Context, _, prompt string) (string, error) {
		return "answered " + prompt, nil
	}
	go func() {
		chat.ServeIPC(ctx, sock, nil, trigger)
	}()
	waitForSock(t, sock)
	cmd := newRootCmd("v0.77.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"chat", "trigger", "my question"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(buf.String(), "answered my question") {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

func waitForSock(t *testing.T, sock string) {
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
