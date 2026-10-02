package settings

import (
	"testing"
)

// Diagnostics is a tool toggle like the others: the loop costs nothing while
// nothing has been edited, so it defaults on and the file can switch it off.
func TestDiagnosticsDefaultOn(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Diagnostics {
		t.Error("diagnostics default off")
	}
}

// The whole precedence chain has to carry the toggle, or the layer that
// turns it on is not the layer that decides.
func TestDiagnosticsFollowThePrecedenceChain(t *testing.T) {
	written(t, "tools:\n  diagnostics: false")
	t.Setenv("BULLE_DIAGNOSTICS", "true")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.Diagnostics {
		t.Error("the environment did not beat the file")
	}
}

// The kill switch is the file layer: diagnostics: false turns the loop off
// against a default that has it on, and reaches the resolved config intact.
func TestDiagnosticsFalseInTheFileTurnsTheLoopOff(t *testing.T) {
	written(t, "tools:\n  diagnostics: false")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.Diagnostics {
		t.Error("diagnostics: false in the file left the loop on")
	}
}
