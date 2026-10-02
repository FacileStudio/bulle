package settings

import (
	"os"
	"testing"
)

func TestLegacyNacelleEnvStillResolves(t *testing.T) {
	t.Setenv("NACELLE_BACKEND", "legacy")
	t.Setenv("BULLE_BACKEND", "current")
	if got := envGet("BACKEND"); got != "current" {
		t.Fatalf("BULLE_ must win over NACELLE_: got %q", got)
	}

	t.Setenv("BULLE_BACKEND", "")
	if err := os.Unsetenv("BULLE_BACKEND"); err != nil {
		t.Fatal(err)
	}
	if got := providerEnv().Backend; got != "legacy" {
		t.Fatalf("NACELLE_BACKEND must still resolve: got %q", got)
	}
}
