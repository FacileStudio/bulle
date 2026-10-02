package ide

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDiscoveryFileNamesTheRunningSession(t *testing.T) {
	srv := startTestServer(t, Options{Root: "/repo", Model: "claude", Version: "v0.76.0", Session: "/tmp/s.jsonl"})
	path := File(os.Getpid())

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the discovery file: %v", err)
	}
	var found Discovery
	if err := json.Unmarshal(raw, &found); err != nil {
		t.Fatalf("parsing the discovery file: %v", err)
	}
	if found.V != Protocol || found.PID != os.Getpid() {
		t.Errorf("discovery = %+v, want version %d for pid %d", found, Protocol, os.Getpid())
	}
	if found.Socket != srv.path || found.Root != "/repo" || found.Session != "/tmp/s.jsonl" {
		t.Errorf("discovery does not describe the server: %+v", found)
	}
	if _, err := time.Parse(time.RFC3339, found.Started); err != nil {
		t.Errorf("started %q is not an RFC 3339 instant: %v", found.Started, err)
	}
}

func TestDiscoveryFileAndItsDirectoryArePrivate(t *testing.T) {
	srv := startTestServer(t, Options{})

	file, err := os.Stat(File(os.Getpid()))
	if err != nil {
		t.Fatalf("statting the discovery file: %v", err)
	}
	if file.Mode().Perm() != 0o600 {
		t.Errorf("discovery file mode = %o, want 600", file.Mode().Perm())
	}
	dir, err := os.Stat(Dir())
	if err != nil {
		t.Fatalf("statting the discovery directory: %v", err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Errorf("discovery directory mode = %o, want 700", dir.Mode().Perm())
	}
	socket, err := os.Stat(srv.path)
	if err != nil {
		t.Fatalf("statting the socket: %v", err)
	}
	if socket.Mode().Perm() != 0o600 {
		t.Errorf("socket mode = %o, want 600", socket.Mode().Perm())
	}
}

func TestRootIsPublishedAsAnAbsolutePath(t *testing.T) {
	srv := startTestServer(t, Options{Root: "."})
	working, err := os.Getwd()
	if err != nil {
		t.Fatalf("reading the working directory: %v", err)
	}
	if srv.opts.Root != working {
		t.Errorf("root = %q, want the absolute %q", srv.opts.Root, working)
	}
}

func TestCloseRemovesTheDiscoveryFileAndTheSocket(t *testing.T) {
	srv := startTestServer(t, Options{})
	file, socket := File(os.Getpid()), srv.path
	if err := srv.Close(); err != nil {
		t.Fatalf("closing the IDE server: %v", err)
	}
	for _, path := range []string{file, socket} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s survived the close: %v", path, err)
		}
	}
	if srv.Close() != nil {
		t.Error("closing the IDE server twice failed")
	}
}

func TestSetSessionUpdatesTheDiscoveryFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(t.TempDir(), "run"))
	srv, err := Start(Options{})
	if err != nil {
		t.Fatalf("starting the IDE server: %v", err)
	}
	defer func() { _ = srv.Close() }()

	if err := SetSession("/tmp/later.jsonl"); err != nil {
		t.Fatalf("recording the session: %v", err)
	}
	raw, err := os.ReadFile(File(os.Getpid()))
	if err != nil {
		t.Fatalf("reading the discovery file: %v", err)
	}
	var found Discovery
	if err := json.Unmarshal(raw, &found); err != nil {
		t.Fatalf("parsing the discovery file: %v", err)
	}
	if found.Session != "/tmp/later.jsonl" {
		t.Errorf("session = %q, want the path set after start", found.Session)
	}
	if got := srv.hello().Fields["session"]; got != "/tmp/later.jsonl" {
		t.Errorf("hello session = %v, want the path set after start", got)
	}
}

func TestSocketPathFollowsTheRuntimeDirectory(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	want := "/run/user/1000/bulle/42.sock"
	if got, err := SocketPath(42); err != nil || got != want {
		t.Errorf("SocketPath = %q, %v, want %q", got, err, want)
	}

	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "/tmp")
	fallback, err := SocketPath(42)
	if err != nil {
		t.Fatalf("SocketPath without a runtime directory: %v", err)
	}
	want = filepath.Join("/tmp", "bulle-"+strconv.Itoa(os.Getuid()), "42.sock")
	if fallback != want {
		t.Errorf("SocketPath fell back to %q, want %q", fallback, want)
	}
}

func TestPackageCloseTakesTheSurfaceDown(t *testing.T) {
	startTestServer(t, Options{})
	if err := Close(); err != nil {
		t.Fatalf("closing the IDE surface: %v", err)
	}
	if _, err := os.Stat(File(os.Getpid())); !os.IsNotExist(err) {
		t.Errorf("the discovery file survived the close: %v", err)
	}
	if err := Close(); err != nil {
		t.Errorf("closing an IDE surface that is already down failed: %v", err)
	}
}
