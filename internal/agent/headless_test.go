package agent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/engine"
	"github.com/FacileStudio/nacelle"
)

func TestStripPrintFlagExtractsPrompt(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		want     string
		wantArgs string
	}{
		{"-print with arg", []string{"bulle", "-print", "hello world"}, "hello world", "bulle"},
		{"-print=equals", []string{"bulle", "-print=hello"}, "hello", "bulle"},
		{"-print alone (stdin)", []string{"bulle", "-print"}, "", "bulle"},
		{"no -print", []string{"bulle", "-model", "abc"}, "", "bulle -model abc"},
		{"-print after other flags", []string{"bulle", "-root", ".", "-print", "hello"}, "hello", "bulle -root ."},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Args
			os.Args = tt.args
			got := stripPrintFlag()
			gotArgs := strings.Join(os.Args, " ")
			os.Args = saved

			if got != tt.want {
				t.Errorf("stripPrintFlag() = %q, want %q", got, tt.want)
			}
			if gotArgs != tt.wantArgs {
				t.Errorf("after strip, os.Args = %q, want %q", gotArgs, tt.wantArgs)
			}
		})
	}
}

func TestConsumeHeadlessEventsDeltasAndStats(t *testing.T) {
	events := make(chan engine.Event, 4)
	events <- engine.NewTextDelta("hello ")
	events <- engine.NewTextDelta("world")
	events <- engine.NewToolCall(&nacelle.ToolEvent{ID: "call_1", Name: "test"})
	events <- engine.NewTurnDone(nacelle.Usage{InputTokens: 10, CacheReadTokens: 5, CacheCreationTokens: 3}, nacelle.Stop("end_turn"))
	close(events)

	var buf bytes.Buffer
	var stats runStats
	target := streamTarget{w: &buf, stats: &stats}
	got, err := consumeHeadlessEvents(context.Background(), events, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world\n" {
		t.Fatalf("got %q, want %q", got, "hello world\n")
	}
	if buf.String() != "hello world\n" {
		t.Fatalf("buf got %q, want %q", buf.String(), "hello world\n")
	}
	if stats.ToolCalls != 1 {
		t.Fatalf("tool calls = %d, want 1", stats.ToolCalls)
	}
	if stats.FinalContextTokens != 18 {
		t.Fatalf("final context tokens = %d, want 18", stats.FinalContextTokens)
	}
}

func TestConsumeHeadlessEventsError(t *testing.T) {
	events := make(chan engine.Event, 1)
	expectedErr := errors.New("stream failed")
	events <- engine.NewError(expectedErr)
	close(events)

	var buf bytes.Buffer
	var stats runStats
	target := streamTarget{w: &buf, stats: &stats}
	got, err := consumeHeadlessEvents(context.Background(), events, target)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if got != "" {
		t.Fatalf("expected empty string on error, got %q", got)
	}
}
