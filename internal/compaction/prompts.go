package compaction

import (
	"fmt"

	"github.com/FacileStudio/nacelle"
)

// SystemPrompt is the system prompt the compaction summarizer writes against.
const SystemPrompt = "You are the compaction engine for a long-running coding agent. Your job " +
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

// CompactAsk is the message tacked onto history turns asking for a summary.
const CompactAsk = "Above are the older turns to compact, and nothing else. Write the compaction " +
	"summary of exactly those turns, following your instructions. The conversation after this " +
	"chunk is kept intact and is not part of this request. Return only the summary block — no " +
	"preamble, no closing remark."

// KeepAsk is the addendum carrying an earlier ledger to extend without repeating.
const KeepAsk = "An earlier " + Sentinel + " is shown below. It already records what " +
	"came before the turns above, so do not repeat anything it holds — write only what those turns " +
	"add. It is shown for that reason alone."

// ConsolidateAsk is the addendum asking to rewrite an overgrown ledger.
const ConsolidateAsk = "Your own earlier " + Sentinel + " has grown past the budget a " +
	"summary may take. It is shown below, and this time it must be rewritten rather than added to: " +
	"one consolidated block covering everything that ledger records and everything the turns above " +
	"add. Keep every load-bearing identifier it names — paths, commands, names, versions, ids — " +
	"verbatim, drop only what is redundant, and leave the whole block under %d tokens."

// Prompt builds the messages fed to the summarizer model.
func Prompt(conv []nacelle.Message, plan []Span, fold Fold, consolidate bool) []nacelle.Message {
	history := fold.LedgerMessages(conv)
	previous := LedgerText(conv, plan)
	if len(history) == 0 {
		previous = ""
	}
	return withAsk(history, AskWith(previous, consolidate))
}

// AskWith builds the specific ask text appended to the history turns.
func AskWith(previous string, consolidate bool) string {
	switch {
	case previous == "":
		return CompactAsk
	case consolidate:
		return CompactAsk + "\n\n" + fmt.Sprintf(ConsolidateAsk, MaxLedgerTokens) + "\n" + previous
	default:
		return CompactAsk + "\n\n" + KeepAsk + "\n" + previous
	}
}

func withAsk(history []nacelle.Message, ask string) []nacelle.Message {
	if len(history) == 0 || history[len(history)-1].Role != nacelle.RoleUser {
		return append(history, nacelle.UserText(ask))
	}
	last := history[len(history)-1]
	last.Parts = append(append([]nacelle.Part{}, last.Parts...), nacelle.Text{Text: "\n\n" + ask})
	history[len(history)-1] = last
	return history
}
