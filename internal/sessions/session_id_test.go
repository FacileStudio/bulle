package sessions

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

func TestGenerateSessionIDFormat(t *testing.T) {
	id := GenerateSessionID()
	if len(id) != 8 {
		t.Fatalf("expected 8 chars, got %d (%q)", len(id), id)
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("expected valid hex string, got %q: %v", id, err)
	}
	second := GenerateSessionID()
	if second == id {
		t.Fatalf("expected distinct IDs, got duplicate %q", id)
	}
}

func TestResolveSessionByPrefixAndPID(t *testing.T) {
	home := t.TempDir()
	xdgtest.Isolate(t, home)

	log := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	id := strings.TrimSuffix(filepath.Base(log.Path()), ".jsonl")
	prefix := id[:4]

	if resolved := ResolveSession(prefix); resolved != log.Path() {
		t.Errorf("expected %q for prefix %q, got %q", log.Path(), prefix, resolved)
	}

	pidStr := strconv.Itoa(os.Getpid())
	if resolved := ResolveSession(pidStr); resolved != log.Path() {
		t.Errorf("expected %q for PID %s, got %q", log.Path(), pidStr, resolved)
	}
}
