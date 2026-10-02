package diff

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/FacileStudio/bulle/internal/toolview"
)

var (
	// addedStyle tints a changed line's text green over a subtle green wash:
	// the pure dark green blended to ~20% onto the pane's grey backdrop.
	addedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Background(lipgloss.Color("#2E412E"))
	// removedStyle tints a changed line's text red over a subtle red wash.
	removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("#412E2E"))
)

// diffPane bundles the render settings every block shares — the content width,
// the gutter width and the block backdrop — so renderBlock stays under the
// package's parameter budget.
type diffPane struct {
	Content  int
	Gutter   int
	Backdrop lipgloss.Style
}

func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return ansi.Truncate(s, limit, "…")
}

// block lays a muted line over the box background, so context, the header and
// the recap share the pane with the changed lines instead of floating on it.
func block(muted lipgloss.Style, transparent bool) lipgloss.Style {
	return toolview.MatchBackground(muted, transparent)
}

// splitLines breaks file contents into diffable lines, dropping a single
// trailing newline — the marker of a well-formed text file, not a line of it.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// cell renders one content line padded to the full pane width, so a box row
// spans width-2 columns and the border plus its margin space are the two
// columns they add.
func cell(base lipgloss.Style, text string, content int) string {
	return base.Width(content).Render(text)
}

// CountChange reports how many lines a change adds and removes, whatever the
// display cap on the diff itself. The recap under a block states the whole
// change, not the window it was shown in.
func CountChange(change EditChange) (added, removed int) {
	if change.Path == "" || change.Before == change.After {
		return 0, 0
	}
	for _, op := range diffOps(splitLines(change.Before), splitLines(change.After)) {
		switch op.kind {
		case '+':
			added++
		case '-':
			removed++
		}
	}
	return added, removed
}

// RenderDiff draws one change as a full-width box: a coloured left border, a
// header naming the file, a numbered gutter up front, the additions in green
// and the removals in red each on their own faint wash, and a recap footer of
// "+x -y". Line numbers are green on additions, red on removals and muted on
// context. borderColor is the caller's verdict on the edit — green or red when
// it finished, the tool's own colour while it is still running.
func RenderDiff(change EditChange, width int, borderColor string, muted lipgloss.Style, transparent bool) string {
	if change.Path == "" || change.Before == change.After {
		return ""
	}
	ops := diffOps(splitLines(change.Before), splitLines(change.After))
	nums := diffLineNums(ops)
	blocks, blockNums := hunks(ops, nums, contextLines)
	if len(blocks) == 0 {
		return ""
	}
	content := max(width-2, 10)
	backdrop := block(muted, transparent)
	pane := diffPane{Content: content, Gutter: gutterWidth(nums), Backdrop: backdrop}

	rows := make([]string, 0, shownDiffLines+2)
	rows = append(rows, cell(backdrop, "  "+change.Path, content))
	shown := 0
	for i, blk := range blocks {
		if i > 0 {
			rows = append(rows, cell(backdrop, strings.Repeat(" ", pane.Gutter+3)+"…", content))
			shown++
		}
		var cut bool
		rows, shown, cut = renderBlock(rows, blk, blockNums[i], pane, shown)
		if cut {
			break
		}
	}
	rows = append(rows, cell(backdrop, "", content))
	rows = append(rows, recap(change, muted, content, transparent))
	return toolview.Box(rows, borderColor, transparent, width)
}

// recap is the box's footer: one margin space, then "+x -y", additions counted
// in green and removals in red, sharing the block background with the header.
// Each figure fragment ends in a full reset, so the space joining them must
// carry the block background itself — a literal space would render bare on the
// terminal's default background, a visibly different patch in the pane.
func recap(change EditChange, muted lipgloss.Style, content int, transparent bool) string {
	added, removed := CountChange(change)
	var parts []string
	if added > 0 {
		parts = append(parts, toolview.MatchBackground(lipgloss.NewStyle().Foreground(lipgloss.Color("10")), transparent).Render("+"+strconv.Itoa(added)))
	}
	if removed > 0 {
		parts = append(parts, toolview.MatchBackground(lipgloss.NewStyle().Foreground(lipgloss.Color("9")), transparent).Render("-"+strconv.Itoa(removed)))
	}
	gap := block(muted, transparent).Render(" ")
	return cell(block(muted, transparent), " "+strings.Join(parts, gap), content)
}

// renderBlock appends one hunk's rows to the box, stopping at the display cap
// with a marker saying the diff was cut rather than pretending it wasn't. It
// returns the grown slice, how many rows are through, and whether the cap was
// reached.
func renderBlock(rows []string, blk []diffOp, nums []int, pane diffPane, shown int) ([]string, int, bool) {
	for i, op := range blk {
		if shown >= shownDiffLines {
			rows = append(rows, cell(pane.Backdrop, strings.Repeat(" ", pane.Gutter+3)+"… more", pane.Content))
			return rows, shown, true
		}
		var row string
		switch op.kind {
		case '-':
			row = cell(removedStyle, gutter(nums[i], pane.Gutter, "-")+truncate(op.text, pane.Content-pane.Gutter-3), pane.Content)
		case '+':
			row = cell(addedStyle, gutter(nums[i], pane.Gutter, "+")+truncate(op.text, pane.Content-pane.Gutter-3), pane.Content)
		default:
			row = cell(pane.Backdrop, gutter(nums[i], pane.Gutter, " ")+truncate(op.text, pane.Content-pane.Gutter-3), pane.Content)
		}
		rows = append(rows, row)
		shown++
	}
	return rows, shown, false
}
