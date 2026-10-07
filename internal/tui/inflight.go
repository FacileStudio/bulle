package tui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/approval"
	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/bulle/internal/herdr"
	"github.com/FacileStudio/bulle/internal/toolview"
	"github.com/FacileStudio/nacelle"
)

// answerStream holds the text buffers produced during a run. Embedded in
// inflight so every field still reads as m.run.answer, m.run.fullAnswer, etc.
type answerStream struct {
	answer        strings.Builder
	fullAnswer    strings.Builder
	committedLen  int
	reasoning     strings.Builder
	reasoningFull strings.Builder
	// liveOut is an estimate of the output tokens streamed since the last
	// turn boundary, from the deltas themselves. The stream reports real
	// usage only when a turn ends, so this is what lets the output and
	// context counters tick while the model is still writing; it is cleared
	// the moment the authoritative per-turn usage lands.
	liveOut int64
}

// runControl holds the run's coordination state. Embedded in inflight so every
// field still reads as m.run.results, m.run.cancel, etc.
type runControl struct {
	results <-chan result
	cancel  context.CancelFunc
	usage   nacelle.Usage
	stop    nacelle.Stop
	busy    bool
	pending *approval.Request

	// compactChan is the open compaction pass's outcome channel, nil when no
	// pass is in flight. compaction.Outcome lives in compact.go, same package.
	compactChan <-chan compaction.Outcome

	// bgCtx is the run's background context, kept on the run so a compaction
	// that finishes before the model starts can pass it back to startRun.
	bgCtx context.Context

	// overflow is the context-length rejection that ended a run and has not
	// been answered yet, nil otherwise. It is held rather than reported so the
	// run can end quietly: settle is the one place that knows whether a pass
	// can still rescue the turn, and a red failure line the client is about to
	// fix is worse than no line at all. overflowTried is whether this turn has
	// already spent its one retry, which is what stops a provider that refuses
	// even the compacted conversation from looping through passes forever.
	overflow      error
	overflowTried bool
}

// failureCollapse tracks consecutive identical tool failures so they
// render as one line with a count rather than as N identical two-line
// blocks. box holds the boxed detail of the first failure, so a collapsed run
// still draws its red box once. Cleared at the same edges as the rest of a
// run's state: flush in stranded and settle, reset in clearGroups.
type failureCollapse struct {
	toolLine string
	name     string
	err      string
	duration time.Duration
	count    int
	box      string
}

// inflight is the one run this client allows at a time: how to hear from it,
// how to abandon it, what it has produced, and what it has cost.
type inflight struct {
	runControl
	answerStream
	editState
	clock
	turn
	groups        []toolview.Group
	groupIndex    map[string]int
	failures      failureCollapse
	promptEditKey string
}

// clock is when this run started and when esc was first pressed this one.
type clock struct {
	began       time.Time
	interrupted time.Time
	turnBegan   time.Time
}

// turn is the assistant turn being built for the conversation: the tools it
// asked for, and the results collected to answer them.
type turn struct {
	asked    []nacelle.Part
	answered []nacelle.Part
	reported bool
}

// editState is what a run tracks per tool call, plus what drawing a diff for
// its file edits needs: the directory the file tools work in, whether diffs
// were asked for at all, the before/after of every editing call in flight
// keyed by call id, and the raw result string of each call so a boxed
// run_command's output can be drawn alongside its file diff. Both maps live
// here rather than on the model because a run's edits and outputs are not a
// property of the client.
type editState struct {
	root       string
	diffs      bool
	edits      map[string]diff.EditChange
	outputs    map[string]string
	editorPath string
}

// beginTool turns a call into a group row. A new call either extends the
// previous group — same tool kind, and the previous one has not returned yet —
// or starts a fresh row. Grouping by kind aggregates consecutive calls of the
// same category (read, write, network, delegate) even when the specific tool
// or input differs, showing a combined line like "⏺ 4 commands · cmd1 · cmd2 · …".
func (r *inflight) appendToLastGroup(ev nacelle.ToolEvent) bool {
	if len(r.groups) == 0 {
		return false
	}
	g := &r.groups[len(r.groups)-1]
	if !g.End.IsZero() || toolview.ToolKind(ev.Name, ev.Source) != toolview.ToolKind(g.Tool.Name, g.Tool.Source) {
		return false
	}
	g.Count++
	g.CallNames = append(g.CallNames, toolview.PrimaryArg(ev.Input))
	if ev.ID != "" {
		g.CallIDs = append(g.CallIDs, ev.ID)
		r.groupIndex[ev.ID] = len(r.groups) - 1
	}
	return true
}

func (r *inflight) beginTool(ev nacelle.ToolEvent, groupTools bool) {
	if r.groups == nil {
		r.groups = make([]toolview.Group, 0, 4)
	}
	if r.groupIndex == nil {
		r.groupIndex = make(map[string]int, 4)
	}
	if groupTools && r.appendToLastGroup(ev) {
		return
	}
	g := toolview.Group{
		Name:      ev.Name,
		Input:     ev.Input,
		Count:     1,
		Tool:      ev,
		Start:     time.Now(),
		CallNames: []string{toolview.PrimaryArg(ev.Input)},
	}
	if ev.ID != "" {
		g.CallIDs = []string{ev.ID}
	}
	r.groups = append(r.groups, g)
	if ev.ID != "" {
		r.groupIndex[ev.ID] = len(r.groups) - 1
	}
}

// finishTool marks a group's row as returned, with the outcome and the
// duration. It looks the group up by ID rather than by position, because a
// result can arrive for a call that was not the most recent one — the model
// fires several reads in parallel and they land back in whatever order the
// filesystem gives.
func (r *inflight) finishTool(ev nacelle.ToolEvent) {
	if ev.ID == "" {
		for _, g := range slices.Backward(r.groups) {
			if g.End.IsZero() {
				g.FinishCall(ev)
				return
			}
		}
		return
	}
	if i, ok := r.groupIndex[ev.ID]; ok && i < len(r.groups) {
		r.groups[i].FinishCall(ev)
	}
}

func (r *inflight) findGroup(id string) *toolview.Group {
	if id == "" {
		return nil
	}
	i, ok := r.groupIndex[id]
	if !ok || i >= len(r.groups) {
		return nil
	}
	return &r.groups[i]
}

func (r *inflight) heldLine(id string, width int) (string, bool) {
	g := r.findGroup(id)
	if g == nil {
		return "", false
	}
	return g.GroupLine(width), true
}

func (r *inflight) isGroupComplete(id string) bool {
	g := r.findGroup(id)
	return g == nil || g.Count <= 1 || g.FinishedCount >= g.Count
}

func (r *inflight) clearGroups() {
	r.groups = nil
	r.groupIndex = nil
}

// parkApproval holds an incoming approval request until the next keypress
// calls decide, then hands it back as nil so the loop stays awake for that
// press. An attached editor is asked as well, and its answer arrives on this
// loop the same way a keypress does: whichever of the two answers first
// decides. A refusal from the editor — a click, a deadline, a lost connection,
// or the session ending — is a refusal, never an implicit yes; an editor that
// was never attached is asked nothing, and this terminal prompt is the decision
// on its own.
func (m *Model) parkApproval(req approval.Request) tea.Cmd {
	m.run.pending = &req
	herdr.Report(m.herdrClient, herdr.Blocked)
	return m.askEditor(req)
}
