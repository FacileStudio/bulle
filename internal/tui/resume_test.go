package tui

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/sessions"
)

func TestInitReplaysResumedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := sessions.OpenSession("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	log.Line(sessions.FromReader, "hello from user")
	log.Line(sessions.FromModel, "hello from assistant")

	m := NewModel(nil, "test · model", nil, SessionConfig{
		Resume:            log.Path(),
		PromptPlaceholder: "placeholder",
	})
	m.Init()

	if len(m.conversation) != 2 {
		t.Fatalf("conversation length = %d, want 2", len(m.conversation))
	}
	if said(m.conversation[0]) != "hello from user" {
		t.Errorf("msg 0 = %q, want 'hello from user'", said(m.conversation[0]))
	}
	if said(m.conversation[1]) != "hello from assistant" {
		t.Errorf("msg 1 = %q, want 'hello from assistant'", said(m.conversation[1]))
	}

	foundResumeNotice := false
	for _, line := range m.unprinted {
		if strings.Contains(line, "resumed session") && strings.Contains(line, "2 messages") {
			foundResumeNotice = true
			break
		}
	}
	if !foundResumeNotice {
		t.Errorf("expected resumed session notice in unprinted lines, got %v", m.unprinted)
	}
}

func TestDetachCmdSetsDetached(t *testing.T) {
	m := sized()
	cmd := m.detachCmd()
	if !m.detached {
		t.Error("expected m.detached to be true after detachCmd")
	}
	if cmd == nil {
		t.Error("expected non-nil tea.Quit cmd from detachCmd")
	}
}
