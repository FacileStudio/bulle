package tui

import (
	"strings"

	"github.com/FacileStudio/bulle/internal/diff"
	"github.com/FacileStudio/bulle/internal/layout"
	"github.com/FacileStudio/bulle/internal/toolview"
	"github.com/FacileStudio/nacelle"
)

// commandLineCap bounds how many rows of a run_command's output one box shows,
// before a marker says the rest was cut. A command's whole output can be huge;
// a glance at the tail plus the marker is enough without flooding scrollback.
const commandLineCap = 200

// boxBorder is the left-spine colour of a finished edit or command box: green
// for a call that worked, red for one that failed. The codes are the basic
// palette lipgloss reads as those hues — the raw SGR 32/31 escapes the tool
// lines already wear would land in the ANSI256 range and render as desaturated
// teals. A still-running box wears yellow via toolview.ToolBorder instead.
func boxBorder(ok bool) string {
	if ok {
		return "2"
	}
	return "1"
}

// finishEdit renders the boxed detail of one finished call — its file diff,
// its run_command output, or both — and hands it to the transcript. ok is the
// call's outcome and colours the left border.
func (m *Model) finishEdit(id string, tool *nacelle.ToolEvent, ok bool) {
	change, hasChange := m.run.edits[id]
	rawOut := m.run.outputs[id]
	box := m.editBoxFor(id, tool, ok)
	if box == "" {
		return
	}
	m.say(fromDiff, box)
	if last := len(m.held) - 1; last >= 0 {
		if hasChange {
			m.held[last].diff = &change
		}
		m.held[last].ok = ok
		if tool.Name == "run_command" {
			m.held[last].out = rawOut
		}
	}
}

// editBoxFor builds the box(es) a finished call shows and forgets the call's
// pending data. It is used by both the success path and, stored in the failure
// collapse, the first of a run of identical failures — so a collapsed run
// draws its box once, not once per identical error.
func (m *Model) editBoxFor(id string, tool *nacelle.ToolEvent, ok bool) string {
	change, edited := m.drainEdit(id)
	result, has := m.run.outputs[id]
	if has {
		delete(m.run.outputs, id)
	}
	var box strings.Builder
	if edited {
		if d := diff.RenderDiff(change, m.width, boxBorder(ok), m.theme.Muted, m.transparent); d != "" {
			box.WriteString(d)
		}
	}
	if tool.Name == "run_command" && has {
		box.WriteString(m.outputBox(result, ok))
	}
	return box.String()
}

// drainEdit takes and clears the captured change for a call, applying the
// prior-contents fallback a run_command or overwritten file needs.
func (m *Model) drainEdit(id string) (diff.EditChange, bool) {
	change, edited := m.run.edits[id]
	if !edited {
		return change, false
	}
	delete(m.run.edits, id)
	if change.After == "" && change.Before != "" {
		change.After = diff.PriorContents(m.run.root, change.Path)
	}
	return change, true
}

// boxedGroupRow is the single pane row a running edit or command draws in the
// live region: its held line inside the same block background its result will
// fill, so the box is continuous from "running" to "done".
func (m *Model) boxedGroupRow(g toolview.Group) string {
	line := g.InFlightLine(m.width)
	if line == "" {
		return ""
	}
	content := max(m.width-2, 10)
	return toolview.MatchBackground(m.theme.Muted, m.transparent).Width(content).Render(layout.Truncate(toolview.ToolLineRunning(line), content))
}

// inFlightGroup draws one running tool's live row — boxed for an edit or
// command with the running yellow on the left border while it runs, and as
// the ordinary held line, also yellow, for every other tool. A running
// command's streamed output fills the box beneath its line as the lines
// arrive.
func (m *Model) inFlightGroup(g toolview.Group) string {
	if !diff.IsEditTool(g.Name) {
		line := g.InFlightLine(m.width)
		if line == "" {
			return ""
		}
		return toolview.ToolLineRunning(line)
	}
	var rows []string
	if row := m.boxedGroupRow(g); row != "" {
		rows = append(rows, row)
	}
	rows = append(rows, m.liveOutputRows(g)...)
	if len(rows) == 0 {
		return ""
	}
	return toolview.Box(rows, toolview.ToolBorder(), m.transparent, max(m.width, 10))
}

// liveOutputRows are the streamed output lines of a running run_command, ready
// to sit in the box under its held line. They only apply to a single call (a
// batched group has no one output to name), and read the buffer absorbToolOutput
// fills, so a backend that streams lets the box grow while the command runs.
func (m *Model) liveOutputRows(g toolview.Group) []string {
	if g.Name != "run_command" || g.Count != 1 {
		return nil
	}
	text := m.run.outputs[g.Tool.ID]
	if text == "" {
		return nil
	}
	content := max(m.width-2, 10)
	base := toolview.MatchBackground(m.theme.Plain, m.transparent).Width(content)
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	rows := make([]string, 0, min(len(lines), commandLineCap))
	for i, ln := range lines {
		if i >= commandLineCap {
			break
		}
		rows = append(rows, base.Render(layout.Truncate(layout.Unstyled(strings.ReplaceAll(ln, "\r", "")), content-1)))
	}
	return rows
}

// outputBox renders a run_command's raw output as a full-width box sharing the
// diff's background, so a command's stdout and stderr are as visible as the
// file edits around them. The border follows the same verdict as the diffs.
func (m *Model) outputBox(result string, ok bool) string {
	if result == "" {
		return ""
	}
	content := max(m.width-2, 10)
	base := toolview.MatchBackground(m.theme.Plain, m.transparent).Width(content)
	lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")
	rows := make([]string, 0, min(len(lines), commandLineCap)+1)
	for i, ln := range lines {
		if i >= commandLineCap {
			rows = append(rows, base.Render("… more"))
			break
		}
		rows = append(rows, base.Render(layout.Truncate(layout.Unstyled(strings.ReplaceAll(ln, "\r", "")), content-1)))
	}
	return toolview.Box(rows, boxBorder(ok), m.transparent, max(m.width, 10))
}
