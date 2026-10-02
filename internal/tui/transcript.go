package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/theme"
	"github.com/FacileStudio/bulle/internal/toolview"
)

// speaker is who a line on screen belongs to, which is all the drawing needs
// to know about it.
type speaker int

const (
	// fromClient is the client talking about itself: the banner, and what it
	// has to say about its own state.
	fromClient speaker = iota

	// fromReader is the question that was typed.
	fromReader

	// fromModel is the answer.
	fromModel

	// fromThinking is the model's reasoning.
	fromThinking

	// fromTool is a tool the model asked for.
	fromTool

	// fromResult is that tool having finished.
	fromResult

	// fromDiff is the before/after of a file edit, styled line by line by the
	// renderer that produced it rather than painted again here.
	fromDiff

	// fromFailure is the run falling over.
	fromFailure

	// fromTurn is the boundary line closing an assistant turn.
	fromTurn

	// fromCompact is the client reporting that it compacted the context.
	fromCompact

	// fromStart is the configured start message, which paints in the
	// terminal's own foreground rather than the muted client grey.
	fromStart
)

// say commits one finished thing to the terminal's own scrollback.
//
// Nothing is kept. This client used to hold every line in a viewport and
// redraw the lot on each arriving character, which is what made it the only
// thing that could scroll them — and the reason the terminal could not. A
// finished line is printed once, above the live region, and belongs to the
// terminal from then on: its scrollback, its selection, its search, tmux's
// copy-mode. See View for why that is worth more than owning them.
//
// The line is queued rather than printed here because printing is a Cmd, and
// say has callers that cannot return one — absorb folding an event, flush
// committing an answer. Update drains the queue after every message, so the
// order lines are said in is the order they land in.
//
// Spacing is not each line's business: paint arms and renderers emit whatever
// edge newlines they happen to emit, so say trims them all off and prints
// joins the queue with exactly one blank row between lines. Uniform spacing,
// whatever said it.
//
// The session log records the same text tagged with who said it, so a /resume
// replays the conversation rather than the display. fromStart is the one
// speaker that paints but is not recorded — the start message is client
// chrome, not part of the conversation.
func (m *Model) say(who speaker, text string) {
	m.recordHeld(who, text)
	painted := m.paint(who, text)
	m.unprinted = append(m.unprinted, strings.Trim(painted, "\n"))
	m.session.Line(sessions.Speaker(who), text)
}

// prints hands everything said since the last message to the terminal, as a
// single Cmd, and forgets it.
//
// One Println for the batch rather than one per line: tea.Batch makes no
// promise about the order its commands run in, and a transcript delivered out
// of order is not a transcript. Joining them first makes the whole batch one
// message, which insertAbove writes in one go.
//
// One message, but not necessarily one Println — see printed, which cuts a
// batch taller than the window has room for into pieces that still arrive in
// order. Joining here and splitting there is deliberate: the split is a
// property of the screen, and nothing about what was said should have to know
// how tall the terminal is.
func (m *Model) prints() tea.Cmd {
	if len(m.unprinted) == 0 {
		return nil
	}
	said := m.unprinted[0]
	for _, e := range m.unprinted[1:] {
		said += m.gap(said, e) + e
	}
	m.unprinted = nil
	if m.saidOnce {
		said = "\n" + said
	}
	m.saidOnce = true
	return m.printed(said)
}

// paint is how one line looks.
//
// Nobody is labelled. A transcript prefixing every line with who said it
// spends the left margin on something the styling already says, and reads
// like a chat log rather than like a session. The reader's own question is
// the thing they scroll back to find, so that is what gets a muted background
// plus the same left half-block spine (▌) the tool boxes and the prompt draw,
// reading as a bordered pane; the answer is the thing being read, so it gets
// none, and is rendered as the markdown the model almost certainly wrote it
// in.
//
// What the client says about itself is the one thing not held to a width,
// and the banner is why. It is painted in newModel before any WindowSizeMsg
// has arrived, so the only width available is the 80 the model starts at —
// and it is now printed to stdout before the program starts, so it is never
// repainted either. Unconstrained, the terminal wraps it the way it wraps
// everything else, which is what this file argues for everywhere else.
//
// Width is taken once, here, at the moment the line is printed. It can never
// be re-taken: the line is in the terminal's scrollback from then on, and a
// resize reflows it the way the terminal reflows everything else rather than
// the way this client would. That is the one thing owning a viewport bought
// that this gives up, and it is worth it — every other tool in the terminal
// behaves this way, including the shell the client was launched from.
func (m *Model) paint(who speaker, text string) string {
	width := max(m.width, 1)
	switch who {
	case fromReader:
		return m.question(text, width)
	case fromModel:
		return m.answerBlock(text, width)
	case fromThinking:
		return m.margined([]string{m.theme.Thinking.Width(width - 2).Render(text)})[0]
	case fromTool:
		return toolview.ToolLinePainted(text)
	case fromResult:
		return m.theme.Result.Width(width).Render("⤷ " + text)
	case fromDiff:
		return text
	case fromFailure:
		return m.theme.Failure.Width(width).Render(text)
	case fromTurn:
		return " " + m.theme.Muted.Render(text)
	case fromCompact:
		return m.theme.Compacting.Render(text)
	case fromStart:
		return text
	default:
		return " " + m.theme.Client.Render(text)
	}
}

// streaming is what a run has produced but not finished: the reasoning and
// the answer as they arrive, drawn in the live region under everything
// already printed.
//
// It is tailed to the rows the window can spare rather than shown whole. The
// live region is repainted on every delta, so it has to fit on the screen —
// an answer longer than the terminal cannot be redrawn in place at all, and
// trying is how an inline program corrupts its own output. What scrolls off
// the top is not lost: the whole answer is printed, rendered, the moment it
// finishes.
//
// The answer is rendered through the markdown renderer live, because reading
// raw asterisks in the streaming region is worse than a slight reflow when a
// new character arrives. Half a code block falls back to plain text — glamour
// is lenient with incomplete markdown.
func (m *Model) streaming() []string {
	var live []string
	if reasoning := m.run.reasoning.String(); reasoning != "" {
		m.Stamp()
		block := m.theme.Thinking.Render(m.Collapsed(m.Elapsed()))
		if m.Expanded {
			block = m.theme.Thinking.Width(max(m.width, 1)).Render(reasoning)
		}
		live = append(live, block)
	}

	if answer := m.run.answer.String(); answer != "" {
		live = append(live, m.answerPane(answer, max(m.width, 1)))
	}
	groups := m.inFlightGroups()
	if len(groups) > 0 && len(live) > 0 {
		live = append(live, "")
	}
	live = append(live, groups...)
	if len(live) == 0 {
		return nil
	}

	return strings.Split(strings.Join(live, "\n"), "\n")
}

// inFlightGroups renders every tool group still running as a row the live
// region redraws each frame. A finished group is printed once and belongs to
// the terminal — only the still-open ones can grow. A compaction pass is drawn
// as a row of its own, in the same purple working() uses, so the summarizer
// shows up live in the conversation rather than only in the status line.
func (m *Model) inFlightGroups() []string {
	var groups []string
	if m.compacting {
		elapsed := m.spin.View() + " ✂ compacting session"
		if !m.compactBegan.IsZero() {
			elapsed += " · " + max(time.Since(m.compactBegan).Round(time.Millisecond), time.Millisecond).String()
		}
		groups = append(groups, m.theme.Compacting.Render(elapsed))
	}
	for _, g := range m.run.groups {
		if !g.End.IsZero() {
			continue
		}
		if row := m.inFlightGroup(g); row != "" {
			groups = append(groups, row)
		}
	}
	return groups
}

func (m *Model) restyle() {
	m.pretty = theme.Prettier(m.theme.Markdown, max(m.width-2, 1))
}

func (m *Model) markdown(text string) string {
	return strings.Trim(theme.RenderMarkdown(m.pretty, text), "\n")
}
