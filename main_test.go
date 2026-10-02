package main

import (
	"errors"
	"runtime/debug"
	"testing"
)

func TestTheLibraryPrefixIsNotPrintedTwice(t *testing.T) {
	if got := unprefixed(errors.New("bulle: backend \"anthropic\" does not support x")); got != `backend "anthropic" does not support x` {
		t.Errorf("unprefixed = %q, want the library's own name dropped", got)
	}
	if got := unprefixed(errors.New("nacelle/openrouter: no API key")); got != "nacelle/openrouter: no API key" {
		t.Errorf("unprefixed = %q, want a backend's prefix left alone", got)
	}
}

const testedUltraviolet = "v0.0.0-20260703014108-f5a850f9c2b7"

func TestTheInlineRendererIsTheCommitBubbleteaWasTestedAgainst(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build info in this binary, so the pin below went unchecked")
	}
	for _, dep := range info.Deps {
		if dep.Path != "github.com/charmbracelet/ultraviolet" {
			continue
		}
		if dep.Version != testedUltraviolet {
			t.Errorf("ultraviolet = %s, want %s: re-verify the shrink before moving it",
				dep.Version, testedUltraviolet)
		}
		return
	}
	t.Fatal("ultraviolet is not in the build graph")
}
