package tui

import (
	"time"

	"github.com/FacileStudio/bulle/internal/sessions"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/bulle/internal/tasks"
	"github.com/FacileStudio/bulle/internal/usage"
	"github.com/FacileStudio/nacelle"
)

// account holds what the client knows about what the session has spent: the
// usage total, and what it knows about how much context the conversation is
// carrying.
//
// The two live together because they answer one question from two sides — what
// the session cost, and whether the conversation is now too heavy to continue
// as-is — and both outlive any single run.
type account struct {
	// spent is the session's cumulative usage; the status line adds the run
	// in flight to it into a total that only ever goes up.
	spent nacelle.Usage

	// rate is the realised cost per token of the most recent turn that
	// reported one: its Cost divided by its total billed tokens. The status
	// line multiplies the live output-token estimate by it while a turn
	// streams, so the dollar figure ticks without waiting for the turn to
	// end. It stays zero until a turn reports a Cost — and stays that way on
	// backends that never report one, so no dollars are invented — and the
	// authoritative per-turn Cost replaces the estimate at the next turn
	// boundary.
	rate float64

	// size is the input cost of the most recent finished turn, in tokens.
	// Every turn re-bills the whole conversation as input, so this is also
	// what the conversation would cost to send again right now — measured,
	// by the backend's own accounting, not guessed from bytes.
	size int64

	// trimmed is how many history tool results have been masked. It reaches the
	// status line, because a model whose memory was quietly edited should not be
	// the only one who knows.
	trimmed int

	// compactBegan is when the current compaction pass started, stamped in
	// beginCompaction. The running-tool row draws its elapsed time against
	// it, the same way every other live row draws against its group's start.
	compactBegan time.Time

	// began is when this client started, stamped once in newModel. The
	// status line and the closing recap both measure against it, so the
	// two can never disagree about how long the session ran.
	began time.Time

	// tools and failed are how many tool calls this session finished, and
	// how many of those fell over. They are counted where a call ends
	// rather than derived at the end from anything, because nothing keeps
	// a record of a finished call: the line is printed and forgotten.
	tools  int
	failed int

	// sink appends each finished turn to mycelium's event feed, so a session
	// running here shows up in mycelium's dashboard while it is still going.
	// It is nil when mycelium is not installed on this machine.
	sink *usage.Sink

	// session is this run of the client written down: the questions asked
	// and the answers given, appended to a file under ~/.bulle/sessions
	// as they are said. It is nil when the file could not be opened, which
	// is not a reason to refuse to run.
	session *sessions.SessionLog

	// tasks is the plan the model is working to, as it last reported it.
	// It is written only by the routed update, never by the tool that
	// produces it — see tasks.go for why a tool goroutine cannot touch it.
	tasks tasks.TaskList

	// grind is the per-run minimum spend this session demands, and how many
	// continuations the current run has used of it. With no floor set it is
	// off and nothing here ever fires.
	grind grindBudget
}

// total is the session's spend: every finished run plus what the run in flight
// has reported so far. It is the one figure the status line, /status and the
// recap all read, so they cannot disagree about what the session cost.
func (m *Model) total() nacelle.Usage {
	return m.spent.Add(m.run.usage)
}

// tokenTotals renders the input and output counts the footer, /status and the
// run recap share. Input counts cache creations, which are billed like any
// other input; cache reads are left to the surfaces that name them, so the two
// figures are never read as one.
func tokenTotals(u nacelle.Usage) string {
	return "↑" + status.ShortTokens(u.InputTokens+u.CacheCreationTokens) + " ↓" + status.ShortTokens(u.OutputTokens)
}

// sized records what a finished turn cost on the input side.
//
// Cache reads and cache creations are billed input like any other, and both
// backends report them here, so leaving either out would understate the
// conversation by most of an agentic session's real size.
//
// It is only ever called with a turn's own usage. KindDone carries the run's
// total — the sum of every turn's input, the conversation billed once per turn
// — which is a bill, not a conversation, and reading it as a size would show a
// three-turn run as three times the context it actually holds.
//
// A usage that reports no input at all is left as it stands rather than written
// through. Zero input is what a backend that does not report usage on every
// event looks like, and taking it at face value would erase the one measure both
// automatic triggers read — compaction would stand down silently, which is the
// same failure the pre-send guard had when it treated an uncountable backend as
// nothing to do. A conversation of any size never bills zero input tokens, so
// the guard cannot hide a real reading: it only refuses to remember a number
// that means "unknown" as if it meant "empty".
func (m *Model) sized(usage nacelle.Usage) {
	spent := usage.InputTokens + usage.CacheReadTokens + usage.CacheCreationTokens
	if spent <= 0 {
		return
	}
	m.size = spent
}
