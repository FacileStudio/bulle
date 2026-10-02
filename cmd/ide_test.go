package cmd

import (
	"bytes"
	"testing"

	"github.com/FacileStudio/bulle/internal/ide"
)

// The --ide switch reaches a session through the pointer the command line wrote
// it into: it is bound once, on the root command, and read by the surface's own
// Enabled rather than carried through the settings a session is built from. A
// flag nothing read would look exactly like this one from the outside.
func TestIDEFlagReachesTheSurfaceSwitch(t *testing.T) {
	t.Cleanup(func() { ide.BindFlag(nil) })
	t.Setenv(ide.EnvVar, "")
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--ide", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("parsing --ide: %v", err)
	}
	if !ide.Enabled() {
		t.Error("--ide was parsed but nothing read the switch it set")
	}
}
