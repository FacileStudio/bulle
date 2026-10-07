package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/nacelle"
)

func TestGroupOnlyPrintsOnce(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`}, true)

	if m.run.groups[0].Count != 3 {
		t.Fatalf("group count = %d, want 3", m.run.groups[0].Count)
	}

	said := func() []string { return spoken(m) }

	for _, id := range []string{"a", "b"} {
		m.run.finishTool(nacelle.ToolEvent{ID: id, Name: "read_file", Input: `{"path":"view.go"}`})
		m.finished(&nacelle.ToolEvent{ID: id, Name: "read_file", Input: `{"path":"view.go"}`})
	}
	if lines := said(); len(lines) != 0 {
		t.Errorf("after 2nd finish: %v, want empty transcript", lines)
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"inflight.go"}`})
	lines := said()
	if len(lines) != 1 {
		t.Fatalf("after 3rd finish: %d lines, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "⏺ 3 reads") {
		t.Errorf("group line = %q, want '⏺ 3 reads'", lines[0])
	}
}

func TestGroupIntermediateFailureTrackedAndRendered(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})

	if m.failed != 1 {
		t.Errorf("failed = %d, want 1", m.failed)
	}
	if !m.run.groups[0].End.IsZero() {
		t.Errorf("group end should be zero on intermediate finish")
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`})

	if m.run.groups[0].End.IsZero() {
		t.Errorf("group end should be set after last finish")
	}
	if !m.run.groups[0].Failed {
		t.Errorf("group failed should remain true")
	}
	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "✗ 2 reads") || !strings.Contains(joined, "file not found") {
		t.Errorf("output = %q, want failed group line", joined)
	}
}

func TestGroupLastCallFailureRendered(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`})

	if m.failed != 0 {
		t.Errorf("failed = %d, want 0", m.failed)
	}

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})

	if m.failed != 1 || !m.run.groups[0].Failed {
		t.Errorf("failed = %d, want 1", m.failed)
	}
	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "✗ 2 reads") || !strings.Contains(joined, "permission denied") {
		t.Errorf("output = %q, want failure output", joined)
	}
}

func TestAGroupCallsTrackArgumentsNotNames(t *testing.T) {
	m := bareBanner()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "read_file", Input: `{"path":"run.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "3", Name: "read_file", Input: `{"path":"inflight.go"}`}, true)

	g := m.run.groups[0]
	if g.Count != 3 || len(g.CallNames) != 3 {
		t.Fatalf("group = %+v, want 3 calls", g)
	}
	want := []string{"view.go", "run.go", "inflight.go"}
	for i, w := range want {
		if g.CallNames[i] != w {
			t.Errorf("callNames[%d] = %q, want %q", i, g.CallNames[i], w)
		}
	}
}

func TestGroupLineShowsArguments(t *testing.T) {
	m := bareBanner()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "run_command", Input: `{"command":"ls docs"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "2", Name: "run_command", Input: `{"command":"git status"}`}, true)

	line := m.run.groups[0].GroupLine(m.width)
	if !strings.Contains(line, "ls docs") || !strings.Contains(line, "git status") {
		t.Errorf("group line = %q, want arguments", line)
	}
}

func TestGroupMultipleDistinctFailures(t *testing.T) {
	m := sized()
	m.groupTools = true

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"view.go"}`, Err: errors.New("file not found")})

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "read_file", Input: `{"path":"run.go"}`, Err: errors.New("permission denied")})

	lines := spoken(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "file not found") || !strings.Contains(joined, "permission denied") {
		t.Errorf("output = %q, want both failures", joined)
	}
}

func TestGroupPreservesDiffsOnPartialFailure(t *testing.T) {
	m := sized()
	m.groupTools = true
	m.run.edits = map[string]diff.EditChange{"a": {Path: "view.go", Before: "old", After: "new"}}

	m.run.beginTool(nacelle.ToolEvent{ID: "a", Name: "edit_file", Input: `{"path":"view.go"}`}, true)
	m.run.beginTool(nacelle.ToolEvent{ID: "b", Name: "edit_file", Input: `{"path":"run.go"}`}, true)

	m.run.finishTool(nacelle.ToolEvent{ID: "a", Name: "edit_file", Input: `{"path":"view.go"}`})
	m.finished(&nacelle.ToolEvent{ID: "a", Name: "edit_file", Input: `{"path":"view.go"}`})

	m.run.finishTool(nacelle.ToolEvent{ID: "b", Name: "edit_file", Input: `{"path":"run.go"}`, Err: errors.New("write error")})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "edit_file", Input: `{"path":"run.go"}`, Err: errors.New("write error")})

	if _, ok := m.run.edits["a"]; ok {
		t.Errorf("edit 'a' should be deleted from edits map")
	}
}

func TestSubMillisecondCallFlooredToMillisecond(t *testing.T) {
	if got := status.Took(400_000); got != "1ms" {
		t.Errorf("took(400µs) = %q, want 1ms", got)
	}
}

func TestDiscardedCallExcludedFromTally(t *testing.T) {
	m := bareBanner()

	m.finished(&nacelle.ToolEvent{ID: "a", Name: "read_file", Input: `{"path":"x"}`})
	m.finished(&nacelle.ToolEvent{ID: "b", Name: "run_command", Input: `{"command":"x"}`, Err: errors.New("nope")})
	m.finished(&nacelle.ToolEvent{ID: "c", Name: "read_file", Input: `{"path":"y"}`, Discarded: true})

	if m.tools != 2 {
		t.Errorf("counted %d tools, want 2", m.tools)
	}
	if m.failed != 1 {
		t.Errorf("counted %d failures, want 1", m.failed)
	}
}

func TestTheStatusLineNamesTheToolThatIsRunning(t *testing.T) {
	m := sized()
	m.run.busy = true

	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})

	if status := visible(m.status()); !strings.Contains(status, "running read_file") {
		t.Errorf("status = %q, want running read_file", status)
	}
}

func TestTheStatusLineCountsSeveralToolsAtOnce(t *testing.T) {
	m := sized()
	m.run.busy = true

	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "b", Name: "search_content"}})

	if status := visible(m.status()); !strings.Contains(status, "running 2 tools") {
		t.Errorf("status = %q, want running 2 tools", status)
	}
}

func TestAToolStopsBeingNamedOnceItsResultArrives(t *testing.T) {
	m := sized()
	m.run.busy = true

	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})
	m.absorb(nacelle.Event{Kind: nacelle.KindToolResult, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})

	if status := visible(m.status()); strings.Contains(status, "running read_file") {
		t.Errorf("status = %q, want finished tool no longer running", status)
	}
}

func TestSettleForgetsToolsThatNeverAnswered(t *testing.T) {
	m := sized()
	m.run.cancel = func() {}
	m.run.busy = true
	m.absorb(nacelle.Event{Kind: nacelle.KindToolCall, Tool: &nacelle.ToolEvent{ID: "a", Name: "read_file"}})

	m.settle()

	if n := m.running(); n != 0 {
		t.Errorf("running = %v, want 0", n)
	}
}
