package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/glamour/v2"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/herdr"
	"github.com/FacileStudio/bulle/internal/history"
	"github.com/FacileStudio/bulle/internal/menu"
	"github.com/FacileStudio/bulle/internal/queue"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/bulle/internal/theme"
	"github.com/FacileStudio/nacelle"
)

// commandState is everything /skill:name and the dropdown menu need beyond
// what command.go itself owns: skills to resolve a name against, the
// dropdown's own filter/selection state, and the window height layout()
// needs to reserve the dropdown's own space out of. Embedded rather than
// named, the same reason Config embeds Discovery in config.go: every field
// still reads as m.skills or m.menu, not m.commandState.skills — grouping
// exists only to keep model's own field count from growing by one every
// time this list does.
type commandState struct {
	skills      map[string]skill
	menu        menu.Menu
	modelPicker bool
}

// look is how a line is drawn rather than what it says: the palette resolved
// for the terminal's own background, the markdown renderer built for the
// current width, and the spinner that keeps the status line moving. All three
// are rebuilt or ticked by something other than the thing that produced the
// text, and none of them is ever read without the others nearby.
//
// Embedded, so every field still reads as m.theme, m.pretty and m.spin — the
// grouping exists for the same reason commandState's does, to keep model's own
// field count from growing by one every time this client learns to draw
// something new.
//
// spin is built with no style of its own and has to stay that way. The status
// line renders the spinner, the phrase and the clock as one coloured span, so
// a style here would emit its own reset in the middle of that span and drop
// the colour from everything after the spinner — see working().
type look struct {
	theme          theme.Palette
	pretty         *glamour.TermRenderer
	spin           status.Spinner
	groupTools     bool
	showHooks      bool
	showHookOutput bool
	promptStyles   textarea.Styles
}

// core groups the agent and the startup banner so model stays under filet's
// field cap. Embedded, so every field still reads as m.agent and m.banner.
// delegate is the Config the main agent was built from, kept so a /parallel
// fan-out clones it for its own nested agents.
type core struct {
	agent         *nacelle.Agent
	banner        string
	autoResume    bool
	resumePath    string
	delegate      nacelle.Config
	herdrClient   *herdr.Client
	diagLoop      bool
	detached      bool
	detachedMsg   string
	activeBackend string
	activeModel   string
	activeBaseURL string
	activeAPIKey  string
}

type heldEntry struct {
	who  speaker
	text string
	diff *editChange
	ok   bool
	out  string
}

// transcript groups the conversation, unprinted lines, and transcript-size
// settings so model stays under filet's field cap. Every field still reads as
// m.conversation, m.unprinted, m.compactAt and m.compacting.
type transcript struct {
	conversation []nacelle.Message
	unprinted    []string
	held         []heldEntry
	// hold is the committed, painted transcript in tui mode. Printing to the
	// terminal scrollback is a no-op inside an alternate screen (see tea.Println),
	// so finished lines land here instead and are drawn back into the view each
	// frame, tail-first, with the prompt pinned to the bottom. Each entry is one
	// pre-wrapped row. It is capped at holdRowsCap with the oldest rows dropped
	// ring-buffer style, so a long session cannot grow the per-frame redraw
	// without bound.
	hold []string
	// saidOnce records whether anything has been printed or held yet. Every
	// committed batch after the first gets one leading blank row: spacing
	// between two things said in the same batch comes from the join, but
	// separate flushes print separately and need their own separator.
	saidOnce bool
	// scrollTop is how many transcript rows the scroll wheel has pulled the
	// tui-mode window back from the newest row. Held lines are drawn tail-first
	// with the prompt pinned; scrolling up raises this so earlier rows surface
	// above the live region, scrolling down returns it to 0 (the newest row).
	scrollTop int
	compactAt int64
	// policy is the resolved tier ladder this session compacts by: the ratios,
	// the window they are measured against, the absolute ceiling and the two
	// ends a pass never touches. compactAt stays the absolute figure the gates
	// read; policy is what decides how hard a pass is. judge is the opt-in
	// classifier, nil while it is off — the default.
	policy     compaction.Policy
	judge      compaction.Judge
	compacting bool
	// last is the numbers of the most recent pass of any tier — the soft
	// tombstone, the smart fold, or the mask fallback. /status names the
	// tier from it; the zero value (Below) means no pass has run this session.
	last compacted
	// thrashCount is how many consecutive compaction passes ended with the
	// conversation still over the trigger threshold — a single very large
	// result, usually in the kept tail, that eviction and summarization cannot
	// clear. The automatic triggers back off only once it reaches thrashLimit,
	// so a single failed pass does not disable auto-compaction for the rest of
	// the session. A pass that lands under resets it, and a manual /compact
	// or /clear clears it for a fresh attempt.
	thrashCount int
}

// parallelTaskInfo holds info about a single task in a parallel subagent run.
type parallelTaskInfo struct {
	Task   string
	Result string
	Err    string
	Usage  nacelle.Usage
	Title  string
	Tool   string
	// ToolOut is the running tool's completion state: empty while the call is
	// still going, "ok" when it succeeded, and the error text when it failed.
	// The row colours the tool's glyph from it — tool tone, green, red — while
	// the task itself keeps running.
	ToolOut string
	Began   time.Time
	End     time.Time
	Active  bool
	// Ledgered is how much of this task's usage has already been folded into
	// the session total via live spend updates. A detached fan-out's spend
	// joins m.spent as it streams; finishDetached adds only the residual,
	// so live updates and the final authoritative figure never double-count.
	Ledgered nacelle.Usage
	// Cleared marks a finished task hidden by a /clear. It stays in the batch so
	// running siblings keep their original slice indices — the live-update and
	// result paths address tasks by that position — but the view, the layout
	// row count, and the completion review all skip cleared tasks.
	Cleared bool
}

// parallelState groups the parallel subagent UI state so model stays under
// filet's field cap. A map keyed by tool ID so that overlapping
// parallel_agents calls do not overwrite each other — each call's tasks and
// result are tracked independently. detachedSeq numbers detached /parallel
// batches so they get distinct keys without any shared global: the model-tool
// path keys by nacelle's tool ID, and a model-incrementing counter keeps the
// detached keys apart from them.
type parallelState struct {
	parallelTasks  map[string][]parallelTaskInfo
	detachedSeq    int
	maxConcurrency int
	pending        map[string][]string
	pendingTitles  map[string][]string
}

// composer groups the prompt's own textarea and its recall history, so model
// stays under filet's field cap. Embedded, so every field still reads as
// m.prompt and m.hist.
type composer struct {
	prompt textarea.Model
	hist   *history.History
}

// Model is the whole client: a transcript, a prompt, and at most one run in
// flight.
type Model struct {
	core
	transcript
	queue.Queue

	composer

	account
	look
	commandState
	screen
	thoughts

	parallelState
	run inflight
	ide ideWiring
}
