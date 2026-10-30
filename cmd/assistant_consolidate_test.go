package cmd

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/memory"
)

func TestConsolidateTextCoversDropsOnly(t *testing.T) {
	got := consolidateText(memory.Report{Dropped: []string{"- old"}})
	if !strings.Contains(got, "Oublié") || strings.Contains(got, "Retenu") {
		t.Fatalf("drops only: %q", got)
	}
	both := consolidateText(memory.Report{Added: []string{"- new"}, Dropped: []string{"- old"}})
	if !strings.Contains(both, "Retenu") || !strings.Contains(both, "Oublié") {
		t.Fatalf("both: %q", both)
	}
}
