// Package tui — what the summarizer is asked for: the schema it writes against,
// the ask tacked onto the chunk, and the two addenda that carry an earlier
// ledger. It lives apart from compact_summary.go because the file cap is real and
// the prompt is the one part of a pass that is prose.
package tui

import (
	"fmt"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// compactPrompt is what the summarizer is fed: exactly the history the pass may
// touch, with the ask appended and any earlier ledger carried in that ask so a
// later pass builds on it instead of re-deriving it. The anchor and the active
// window are deliberately excluded, so the model cannot leak the present into
// the past's summary.
//
// The earlier ledger rides in the ask rather than in the history, and that is
// load-bearing twice over. It is never a block the judge can select and never
// history the window walk can drop — the ledger is not summarized away. And this
// call always also carries turns no earlier pass had, which is what makes it
// legitimate to ask for a rewrite of the ledger at all: the rewrite is measured
// against material that was never compressed, rather than a summary of a
// summary. I3b is that second half, and it is enforced here rather than assumed:
// a history with nothing left in it — a plan that has moved past the conversation
// it was measured against — carries no earlier ledger either, so a rewrite is
// never asked for off the ledger alone. The alternative is the shape the
// invariant forbids, a summarizer told to rewrite its own last answer with
// nothing since to rewrite it against.
func compactPrompt(conv []nacelle.Message, plan []compaction.Span, fold compaction.Fold, consolidate bool) []nacelle.Message {
	history := fold.LedgerMessages(conv)
	previous := compaction.LedgerText(conv, plan)
	if len(history) == 0 {
		previous = ""
	}
	return withAsk(history, compactAskWith(previous, consolidate))
}

// compactAskWith is the ask for this pass: the standing ask alone on a first
// pass, the standing ask plus the earlier ledger when there is one, and the
// consolidating addendum when that ledger has outgrown its budget. The two
// additions ask for opposite things on purpose — one is told not to repeat what
// the ledger holds, the other to rewrite it whole — so the ledger's own size is
// what selects between them, not the tier or the pass count.
func compactAskWith(previous string, consolidate bool) string {
	switch {
	case previous == "":
		return compactAsk
	case consolidate:
		return compactAsk + "\n\n" + fmt.Sprintf(consolidateAsk, compaction.MaxLedgerTokens) +
			"\n" + previous
	default:
		return compactAsk + "\n\n" + keepAsk + "\n" + previous
	}
}

// withAsk appends the ask as a user turn, folding it into the last history turn
// when that turn is already the user's — a tool-result reply usually is — so the
// summarizer is never handed two user messages in a row, which the backends
// refuse.
func withAsk(history []nacelle.Message, ask string) []nacelle.Message {
	if len(history) == 0 || history[len(history)-1].Role != nacelle.RoleUser {
		return append(history, nacelle.UserText(ask))
	}
	last := history[len(history)-1]
	last.Parts = append(append([]nacelle.Part{}, last.Parts...), nacelle.Text{Text: "\n\n" + ask})
	history[len(history)-1] = last
	return history
}

// compactSystem is the schema the summarizer writes against. Structured on
// purpose — the research on long-horizon agents is blunt that freeform
// summaries drop the load-bearing details — decisions, constraints, dead ends
// and exact state — that stop a model re-treading them. Verbatim identifiers
// survive so a model can still grep for the file or id a compressed summary
// names. The scoping lines are load-bearing too: the history is handed over
// alone, and the newer turns follow the ledger unchanged, so the prompt must
// stop the model reaching past its chunk.
const compactSystem = "You are the compaction engine for a long-running coding agent. Your job " +
	"is to compress the older turns you are shown into a dense block that preserves everything " +
	"the working model still needs, so it can keep going as if those turns had happened — without " +
	"re-deriving them and without re-doing work. " +
	"Compress, do not reduce to a slogan. This is a compressed handoff of working memory, not a " +
	"prose recap, so keep the sharp edges that cause re-work: " +
	"the actual decisions made and the reasons, not just the conclusion; " +
	"constraints that must still hold — invariants, formats, interface contracts, security rules — " +
	"verbatim when short; " +
	"the state of the work — what exists, what is in flight, what was verified vs assumed; " +
	"dead ends and failed approaches, so the model does not re-try them; and " +
	"load-bearing identifiers verbatim — file paths, package and module names, function, class and " +
	"variable names, command invocations, tool and call ids, message ids, exact error strings, " +
	"version pins, and the config keys and values the work depends on. " +
	"Name the artifacts the work produced or touched, with their paths. " +
	"Structure the summary as short bullet sections, in exactly this order and only these: " +
	"Decisions, Constraints, Plan, State, Artifacts, Ruled out, Open questions. " +
	"Leave a section out if it is empty. Never add prose outside the bullets — no preamble, no " +
	"closing line. " +
	"Never invent facts that are not in the source: no guesses, no reconstructed numbers, no " +
	"unstated intentions. If something is genuinely ambiguous, record it under Open questions " +
	"instead of assuming. " +
	"Summarize only the turns shown to you. The newer turns after this chunk are preserved " +
	"verbatim elsewhere and will follow your summary unchanged, so do not anticipate, reference " +
	"or restate them — your summary must hand off the past without overlapping the present. " +
	"Be as short as correctness allows."

// compactAsk is the message tacked onto the history to ask for the summary. It
// repeats the chunk boundary because the model may lose the system's framing and
// try to \"recap the whole conversation\".
const compactAsk = "Above are the older turns to compact, and nothing else. Write the compaction " +
	"summary of exactly those turns, following your instructions. The conversation after this " +
	"chunk is kept intact and is not part of this request. Return only the summary block — no " +
	"preamble, no closing remark."

// keepAsk is the addendum that carries the earlier ledger on a pass that is only
// adding to it. It says plainly why the ledger is shown — so the model does not
// repeat it — because the natural reading of "here is what you recorded before"
// is to restate it, and a restatement is what doubled the body on every pass. The
// merge refuses the duplicates it can recognise; this is what stops the ones it
// cannot, which are the reworded half.
const keepAsk = "An earlier " + compaction.Sentinel + " is shown below. It already records what " +
	"came before the turns above, so do not repeat anything it holds — write only what those turns " +
	"add. It is shown for that reason alone."

// consolidateAsk is the addendum for a pass whose ledger has outgrown its
// budget: the one case where the summarizer is asked to rewrite the ledger
// instead of adding to it. The turns above are what makes that a question it can
// answer — the call is never handed the ledger alone — and the rewrite is not
// trusted on the strength of the ask: compaction.NextLedger checks it still
// carries every identifier the earlier body named and merges instead if it does
// not.
const consolidateAsk = "Your own earlier " + compaction.Sentinel + " has grown past the budget a " +
	"summary may take. It is shown below, and this time it must be rewritten rather than added to: " +
	"one consolidated block covering everything that ledger records and everything the turns above " +
	"add. Keep every load-bearing identifier it names — paths, commands, names, versions, ids — " +
	"verbatim, drop only what is redundant, and leave the whole block under %d tokens."
