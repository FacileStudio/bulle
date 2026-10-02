package sessions

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestIsPIDRunning(t *testing.T) {
	if !IsPIDRunning(os.Getpid()) {
		t.Errorf("expected current PID %d to be running", os.Getpid())
	}
	if IsPIDRunning(-1) || IsPIDRunning(0) || IsPIDRunning(99999999) {
		t.Errorf("expected invalid PIDs to not be running")
	}
}

func createTestLog(t *testing.T, root string) *SessionLog {
	t.Helper()
	log := newSessionLog("anthropic", "claude-opus-5", root)
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	log.Line(fromReader, "first question")
	log.Line(fromModel, "first answer")
	log.Tool("read_file", 100*time.Millisecond)
	log.Line(fromReader, "second question")
	return log
}

func TestGetSessionAndListSessions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if list := ListSessions(""); list != nil {
		t.Errorf("expected empty sessions list, got %v", list)
	}

	log := createTestLog(t, "/repo/alpha")
	sessions := ListSessions("")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	info := sessions[0]
	if info.PID != os.Getpid() || !info.IsActive || info.Status != StatusRunning {
		t.Errorf("unexpected status or pid: %+v", info)
	}
	if info.LastMessage != "second question" || info.TotalLines != 5 {
		t.Errorf("unexpected message or lines: %+v", info)
	}

	gotByID, err := GetSession(info.ID)
	if err != nil || gotByID.ID != info.ID {
		t.Fatalf("GetSession by ID error: %v", err)
	}

	gotByPath, err := GetSession(log.Path())
	if err != nil || gotByPath.ID != info.ID {
		t.Fatalf("GetSession by path error: %v", err)
	}
}

func TestListSessionsFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	createTestLog(t, "/repo/alpha")

	if len(ListSessions("/repo/alpha")) != 1 {
		t.Errorf("expected 1 session for alpha")
	}
	if len(ListSessions("/repo/beta")) != 0 {
		t.Errorf("expected 0 sessions for beta")
	}
}

func createDeadSessionFile(t *testing.T, dir string, pid int) (string, string) {
	t.Helper()
	filename := "20260101T000000Z-" + strconv.Itoa(pid) + ".jsonl"
	path := filepath.Join(dir, filename)
	header := sessionHeader{
		Version: 1,
		Started: "2026-01-01T00:00:00Z",
		Backend: "openai",
		Model:   "gpt-5",
		Root:    "/test",
		PID:     pid,
	}
	hdrData, _ := json.Marshal(header)
	if err := os.WriteFile(path, append(hdrData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename, path
}

func TestMarkSessionStatusAndDeadProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".bulle", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	filename, _ := createDeadSessionFile(t, dir, 99999998)
	info, err := GetSession(filename)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if info.IsActive || info.Status != StatusCompleted {
		t.Errorf("expected inactive completed session: %+v", info)
	}

	if err := MarkSessionStatus(info.ID, StatusFailed); err != nil {
		t.Fatalf("MarkSessionStatus failed: %v", err)
	}

	info, err = GetSession(info.ID)
	if err != nil || info.Status != StatusFailed {
		t.Fatalf("expected status failed, got %q (err %v)", info.Status, err)
	}
}

func createProcessSessionFile(t *testing.T, dir string, pid int) string {
	t.Helper()
	filename := "20260101T120000Z-" + strconv.Itoa(pid) + ".jsonl"
	path := filepath.Join(dir, filename)
	header := sessionHeader{
		Version: 1,
		Started: "2026-01-01T12:00:00Z",
		Backend: "google",
		Model:   "gemini-2.5",
		Root:    "/proc-test",
		PID:     pid,
	}
	hdrData, _ := json.Marshal(header)
	if err := os.WriteFile(path, append(hdrData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func verifyKilledSession(t *testing.T, filename string) {
	t.Helper()
	sessionAfter, err := GetSession(filename)
	if err != nil || sessionAfter.IsActive || sessionAfter.Status != StatusCompleted {
		t.Fatalf("session not marked completed: %+v, err: %v", sessionAfter, err)
	}
	if err := KillSession(filename); err == nil {
		t.Errorf("KillSession on dead process should return error")
	}
	if err := KillSession("nonexistent-session"); err == nil {
		t.Errorf("KillSession on nonexistent session should error")
	}
}

func TestKillSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("skipping process kill test: %v", err)
	}
	pid := cmd.Process.Pid
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	dir := filepath.Join(home, ".bulle", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	filename := createProcessSessionFile(t, dir, pid)

	if err := KillSession(filename); err != nil {
		t.Fatalf("KillSession failed: %v", err)
	}
	cmd.Wait()
	verifyKilledSession(t, filename)
}

func TestDeleteSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	log := newSessionLog("google", "gemini-2.5", "/test")
	if log == nil {
		t.Fatal("expected non-nil log")
	}
	if err := DeleteSession(log.Path()); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if _, err := GetSession(log.Path()); err == nil {
		t.Errorf("expected error getting deleted session")
	}
	if err := DeleteSession("missing-session"); err == nil {
		t.Errorf("expected error deleting nonexistent session")
	}
}

func TestGetSessionErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := GetSession(""); err == nil {
		t.Errorf("expected error for empty id")
	}
	if _, err := GetSession("missing"); err == nil {
		t.Errorf("expected error for missing session")
	}
	if err := MarkSessionStatus("", StatusCompleted); err == nil {
		t.Errorf("expected error marking empty path")
	}
}
