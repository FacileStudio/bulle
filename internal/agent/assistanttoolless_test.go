package agent

import (
	"context"
	"iter"
	"testing"

	"github.com/FacileStudio/nacelle"
)

type recordingBackend struct {
	nacelle.Backend
	got nacelle.Request
}

func (r *recordingBackend) Name() string { return "recording" }

func (r *recordingBackend) Capabilities() nacelle.Capabilities { return nacelle.Capabilities{} }

func (r *recordingBackend) Stream(_ context.Context, req nacelle.Request) iter.Seq2[nacelle.Event, error] {
	r.got = req
	return func(yield func(nacelle.Event, error) bool) {
		yield(nacelle.Event{Kind: nacelle.KindDone}, nil)
	}
}

func TestToollessAgentMountsNothing(t *testing.T) {
	rec := &recordingBackend{}
	agent, err := nacelle.New(toollessConfig(rec))
	if err != nil {
		t.Fatal(err)
	}
	for range agent.Stream(context.Background(), []nacelle.Message{nacelle.UserText("hi")}) {
	}
	if len(rec.got.Tools) != 0 || len(rec.got.MCP) != 0 || len(rec.got.Hooks) != 0 || rec.got.Approve != nil {
		t.Fatalf("request mounts tools=%d mcp=%d hooks=%d approve=%v",
			len(rec.got.Tools), len(rec.got.MCP), len(rec.got.Hooks), rec.got.Approve != nil)
	}
}
