package tui

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/nacelle"
)

type result struct {
	event nacelle.Event
	err   error
}

// Result carries an event or error from the agent stream.
type Result = result

// finished says the run's channel closed and no more results are coming.
type finished struct{}

// start runs an agent in its own goroutine and hands back the channel its
// results arrive on. The channel closes when the run ends, however it ends,
// which is what lets the update loop tell "still streaming" from "over".
func start(ctx context.Context, agent *nacelle.Agent, conversation []nacelle.Message) <-chan result {
	results := make(chan result)

	go func() {
		defer close(results)
		for event, err := range agent.Stream(ctx, conversation) {
			select {
			case results <- result{event: event, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	return results
}

// waitFor takes exactly one result and re-arms itself from Update.
func waitFor(results <-chan result) tea.Cmd {
	return func() tea.Msg {
		next, open := <-results
		if !open {
			return finished{}
		}
		return next
	}
}

func (m *Model) absorb(event nacelle.Event) {
	if m.run.turnBegan.IsZero() && event.Kind != nacelle.KindTurn && event.Kind != nacelle.KindDone && event.Kind != nacelle.KindToolResult {
		m.run.turnBegan = time.Now()
	}
	m.ideTurn(event.Kind)
	switch event.Kind {
	case nacelle.KindText:
		m.Thought()
		m.introduceReasoning()
		m.run.reported = m.run.reported || event.Text != ""
		m.run.answer.WriteString(event.Text)
		m.run.fullAnswer.WriteString(event.Text)
		m.tickLiveOut(event.Text)
		m.commitParagraphs()
	case nacelle.KindThinking:
		m.run.reasoning.WriteString(event.Text)
		m.tickLiveOut(event.Text)

	case nacelle.KindToolCall:
		m.absorbToolCall(*event.Tool)
	case nacelle.KindToolOutput:
		if event.Tool != nil {
			m.absorbToolOutput(*event.Tool, event.Text)
		}
	case nacelle.KindToolResult:
		m.absorbToolResult(*event.Tool, event.Tool.Result)
	case nacelle.KindTurn:
		m.turn(event)
	case nacelle.KindDone:
		if event.Usage.Total() > 0 {
			m.run.usage = event.Usage
		}
		m.learnRate(event.Usage)
		m.run.stop = event.Stop
		m.run.liveOut = 0
	}
}

// tickLiveOut adds a streamed delta's estimated output tokens to the live
// counter. Both the answer and the reasoning are billed as output, so both
// tick it. ~4 characters per token is the standard heuristic; the figure is a
// running estimate that the authoritative per-turn count replaces when the
// turn ends.
func (m *Model) tickLiveOut(delta string) {
	m.run.liveOut += int64(utf8.RuneCountInString(delta)) / 4
}

// absorbToolOutput accumulates a streamed fragment of a call's output onto the
// call's box buffer, so a running command's live region can fill as its
// output arrives. A fragment is one completed line, emitted without its
// newline — nacelle's line emitter strips it so a consumer sees "one", not
// "one\n" — so each fragment is normalised to end in one and the box's line
// split stays honest. The final KindToolResult still carries the whole
// output; a backend that streams it here first lets the running box show
// progress.
func (m *Model) absorbToolOutput(tool nacelle.ToolEvent, text string) {
	if text == "" {
		return
	}
	if m.run.outputs == nil {
		m.run.outputs = make(map[string]string)
	}
	fragment := text
	if !strings.HasSuffix(fragment, "\n") {
		fragment += "\n"
	}
	m.run.outputs[tool.ID] += fragment
}

func (m *Model) absorbToolCall(tool nacelle.ToolEvent) {
	if tool.Name == "parallel_agents" {
		m.rememberParallelCall(tool)
	}
	m.introduceReasoning()
	m.commitParagraphs()
	m.Thought()
	m.run.reported = true
	m.run.beginTool(tool, m.groupTools)
	if m.run.diffs {
		if change, ok := diff.CaptureEdit(m.run.root, tool.Name, tool.Input); ok {
			m.run.edits[tool.ID] = change
		}
	}
}

func (m *Model) absorbToolResult(tool nacelle.ToolEvent, rawResult string) {
	if tool.Name == "parallel_agents" {
		m.run.finishTool(tool)
		m.finished(&tool)
		m.startDetachedParent(tool, rawResult)
		m.relaxAfterDispatch()
		return
	}
	m.run.finishTool(tool)
	m.finished(&tool)
}

// commitParagraphs hands every finished paragraph of the answer to the
// scrollback as it completes. The boundary is a blank line, not any newline:
// glamour renders each chunk on its own, and a table or list split at a
// single newline arrives as two half-structures that each render mangled.
// A fenced code block with a blank line inside still splits; the tail
// commits whole at the end of the turn, and the live region above shows the
// full buffer meanwhile.
func (m *Model) commitParagraphs() {
	text := m.run.answer.String()
	idx := strings.LastIndex(text, "\n\n")
	if idx < 0 {
		return
	}
	complete := text[:idx+1]
	partial := text[idx+2:]

	m.run.answer.Reset()
	m.run.answer.WriteString(partial)
	m.run.committedLen += len(complete) + 1

	if strings.TrimSpace(complete) != "" {
		m.say(fromModel, complete)
	}
}
