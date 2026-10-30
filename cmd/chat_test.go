package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/xdgtest"
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
	xdgtest.Isolate(t, home)
	sock := filepath.Join(home, "chat.sock")
	t.Setenv("BULLE_CHAT_SOCK", sock)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var gotRoom, gotText string
	send := func(_ context.Context, req chat.IPCRequest) error {
		gotRoom = req.Room
		gotText = req.Text
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
	xdgtest.Isolate(t, home)
	sock := filepath.Join(home, "chat.sock")
	t.Setenv("BULLE_CHAT_SOCK", sock)
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

type fakeChatAdapter struct {
	sent    []chat.Identity
	sendErr error
}

func (f *fakeChatAdapter) Name() string                                      { return "matrix" }
func (f *fakeChatAdapter) Receive(context.Context, func(chat.Message)) error { return nil }
func (f *fakeChatAdapter) Send(_ context.Context, to chat.Identity, _ string) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, to)
	return nil
}

type fakeChatBackend struct {
	reply string
}

func (f *fakeChatBackend) Name() string                                                { return "fake" }
func (f *fakeChatBackend) Capabilities() nacelle.Capabilities                          { return nacelle.Capabilities{} }
func (f *fakeChatBackend) CountTokens(context.Context, nacelle.Request) (int64, error) { return 0, nil }
func (f *fakeChatBackend) Stream(ctx context.Context, _ nacelle.Request) iter.Seq2[nacelle.Event, error] {
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindText, Text: f.reply}, nil)
		yield(nacelle.Event{Kind: nacelle.KindDone, Stop: nacelle.StopEnd}, nil)
	}
}

func TestChatIPCTriggerTargetValidation(t *testing.T) {
	rooms := newRoomRegistry(settings.Defaults(""))
	_, err := rooms.handleIPCTrigger(context.Background(), chatMatrix{}, nil, "", "hello")
	if err == nil || err.Error() != "no room specified and no default room configured" {
		t.Fatalf("expected target error, got: %v", err)
	}
}

func TestChatIPCTriggerNormalizeAndDelivery(t *testing.T) {
	agent, err := nacelle.New(nacelle.Config{Backend: &fakeChatBackend{reply: "agent answer"}, System: "test"})
	if err != nil {
		t.Fatalf("new agent failed: %v", err)
	}
	reg := newRoomRegistry(settings.Defaults(""))
	reg.rooms["matrix:!r:example.org"] = &chatRoom{session: engine.NewSession(agent, engine.NewConversation())}
	adapter := &fakeChatAdapter{}
	reply, err := reg.handleIPCTrigger(context.Background(), chatMatrix{}, adapter, "!r:example.org", "hi")
	if err != nil || reply != "agent answer" || len(adapter.sent) != 1 || adapter.sent[0].Room != "!r:example.org" {
		t.Fatalf("unexpected delivery: reply=%q err=%v sent=%+v", reply, err, adapter.sent)
	}
	adapter.sendErr = errors.New("network down")
	_, err = reg.handleIPCTrigger(context.Background(), chatMatrix{}, adapter, "matrix:!r:example.org", "hi")
	if err == nil || !strings.Contains(err.Error(), "matrix send: network down") {
		t.Fatalf("expected send error, got: %v", err)
	}
}

func TestChatErrorFallbacks(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "fallback.sock")
	t.Setenv("BULLE_CHAT_SOCK", sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()
	go serveMockIPC(ln)
	waitForSock(t, sock)
	if err := runChatSend(context.Background(), "!r:t", []string{"hi"}, nil); err == nil || err.Error() != "chat daemon request failed" {
		t.Fatalf("expected default error, got: %v", err)
	}
	if err := runChatSend(context.Background(), "!r:t", []string{"err"}, nil); err == nil || err.Error() != "custom send error" {
		t.Fatalf("expected custom error, got: %v", err)
	}
	if err := runChatTrigger(context.Background(), "!r:t", []string{"hi"}, nil, &bytes.Buffer{}); err == nil || err.Error() != "chat daemon request failed" {
		t.Fatalf("expected default error, got: %v", err)
	}
}

func serveMockIPC(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		var req chat.IPCRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			conn.Close()
			continue
		}
		resp := chat.IPCResponse{OK: false}
		if req.Text == "err" {
			resp.Error = "custom send error"
		}
		json.NewEncoder(conn).Encode(resp)
		conn.Close()
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
