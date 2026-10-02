package agent

import (
	"testing"

	"github.com/FacileStudio/bulle/internal/ide"
)

// A session that publishes to no editor hands over nothing. The nil check is
// load-bearing rather than tidy: a nil *ide.Server inside the interface is not
// a nil surface, it is an attached editor whose every answer is a refusal, and
// that would turn a session's approvals over to nobody.
func TestASessionWithNoEditorHandsOverNoSurface(t *testing.T) {
	var srv *ide.Server

	if surface := ideSurface(srv); surface != nil {
		t.Errorf("surface = %T, want nothing for a session that publishes to no editor", surface)
	}
}

// A started socket is handed over as the surface the terminal drives, which is
// the one conversion between the socket's command interface and the terminal's.
func TestAStartedSocketIsHandedOverAsASurface(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	srv, err := ide.Start(ide.Options{})
	if err != nil {
		t.Fatalf("starting the surface: %v", err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			t.Errorf("closing the surface: %v", err)
		}
	}()

	if surface := ideSurface(srv); surface == nil {
		t.Error("a started socket was handed over as nothing, so no editor could ever answer")
	}
}
