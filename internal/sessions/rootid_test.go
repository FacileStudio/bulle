package sessions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

func TestTargetSessionLogKeepsTheTargetRootVerbatim(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())

	log := newSessionLog("anthropic", "claude-opus-5", "target:staging")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	if log.root != "target:staging" {
		t.Errorf("root = %q, want the target label kept as-is, not resolved", log.root)
	}

	files := ListSessionFiles("target:staging")
	if len(files) != 1 {
		t.Fatalf("ListSessionFiles(\"target:staging\") = %v, want the one target session", files)
	}
}

func TestListSessionFilesMatchesTargetRootsByIdentity(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())

	targetLog := newSessionLog("anthropic", "claude-opus-5", "target:staging")
	localLog := newSessionLog("anthropic", "claude-opus-5", "/repo")
	if targetLog == nil || localLog == nil {
		t.Fatal("expected non-nil logs")
	}

	for _, tc := range []struct {
		projectRoot string
		want        int
	}{
		{"target:staging", 1},
		{"target:other", 0},
		{"/repo", 1},
		{"/elsewhere", 0},
		{"", 2},
	} {
		if got := len(ListSessionFiles(tc.projectRoot)); got != tc.want {
			t.Errorf("ListSessionFiles(%q) matched %d files, want %d", tc.projectRoot, got, tc.want)
		}
	}
}

func TestListSessionFilesDoesNotFoldTargetRootsIntoTheHostCWD(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if log := newSessionLog("anthropic", "claude-opus-5", "target:staging"); log == nil {
		t.Fatal("expected non-nil log")
	}

	for _, root := range []string{cwd, filepath.Join(cwd, "target:staging")} {
		if files := ListSessionFiles(root); len(files) != 0 {
			t.Errorf("ListSessionFiles(%q) = %v, want none: a target session must not group under the launch directory", root, files)
		}
	}
	if strings.Contains(peekSessionRoot(ListSessionFiles("target:staging")[0]), cwd) {
		t.Error("session header resolved the target root into a host path")
	}
}
