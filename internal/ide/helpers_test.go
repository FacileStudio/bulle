package ide

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

// startTestServer opens a server against temporary home and runtime
// directories, so a test creates nothing on the machine it runs on.
func startTestServer(t *testing.T, opts Options) *Server {
	t.Helper()
	xdgtest.Isolate(t, t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(t.TempDir(), "run"))
	srv, err := Start(opts)
	if err != nil {
		t.Fatalf("starting the IDE server: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv
}

// attach dials a test client and reads the hello it opens with, so a publish
// that follows is known to have an editor to reach.
func attach(t *testing.T, srv *Server) (net.Conn, *Decoder) {
	t.Helper()
	sock, err := net.Dial("unix", srv.path)
	if err != nil {
		t.Fatalf("dialling the IDE socket: %v", err)
	}
	t.Cleanup(func() { sock.Close() })
	dec := NewDecoder(sock)
	readEvent(t, dec)
	return sock, dec
}

// readEvent reads the next object an editor would have been sent.
func readEvent(t *testing.T, dec *Decoder) map[string]any {
	t.Helper()
	var ev map[string]any
	if err := dec.Decode(&ev); err != nil {
		t.Fatalf("reading an event: %v", err)
	}
	return ev
}

// wantString checks one field of an event an editor received.
func wantString(t *testing.T, ev map[string]any, key, want string) {
	t.Helper()
	if got, _ := ev[key].(string); got != want {
		t.Errorf("event field %q = %q, want %q (event: %v)", key, got, want, ev)
	}
}

// wantNumber checks one numeric field of an event an editor received.
func wantNumber(t *testing.T, ev map[string]any, key string, want float64) {
	t.Helper()
	if got, _ := ev[key].(float64); got != want {
		t.Errorf("event field %q = %v, want %v (event: %v)", key, got, want, ev)
	}
}
