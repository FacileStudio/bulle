package ide

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

// TestAStaleDiscoveryFileForAnotherPidIsLeftAlone pins the half of the stale
// rule that lives on this side: a session takes down only its own file, so the
// one a killed process left behind is still there afterwards. Which of them is
// stale is the reader's call, and it decides by checking that the pid is alive.
func TestAStaleDiscoveryFileForAnotherPidIsLeftAlone(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(t.TempDir(), "run"))
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		t.Fatalf("creating the discovery directory: %v", err)
	}
	stale := File(999999)
	if err := os.WriteFile(stale, []byte(`{"v":1,"pid":999999}`+"\n"), 0o600); err != nil {
		t.Fatalf("writing a stale discovery file: %v", err)
	}

	srv, err := Start(Options{})
	if err != nil {
		t.Fatalf("starting the IDE server: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("closing the IDE server: %v", err)
	}

	if _, err := os.Stat(stale); err != nil {
		t.Errorf("a discovery file belonging to another pid was taken down: %v", err)
	}
}

// TestTheSocketDirectoryIsPrivate pins that the socket lives in a directory
// only its owner can reach, since what arrives on it can answer an approval.
func TestTheSocketDirectoryIsPrivate(t *testing.T) {
	srv := startTestServer(t, Options{})

	dir, err := os.Stat(filepath.Dir(srv.path))
	if err != nil {
		t.Fatalf("statting the socket directory: %v", err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Errorf("socket directory mode = %o, want 700", dir.Mode().Perm())
	}
}
