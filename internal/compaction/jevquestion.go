package compaction

import (
	"strings"

	"github.com/FacileStudio/bulle/internal/jev"
)

// state renders the goal and the blocks into the text every question is asked
// against. The goal is the pinned task, so a block is judged by whether the
// original task still needs it rather than by how recent it looks.
//
// The block text is untrusted — it is tool output and file contents, and a block
// can argue for its own verdict. That is why the only destructive decision is
// gated twice in decide (the prune probability and the calibrated confidence),
// and why anything unclear falls back to Keep. Widening what is sent here, or
// what one verdict is allowed to do, means revisiting that gate rather than this
// function.
//
// Every piece of text that reaches the request is cut here or before it: each
// block arrives already cut to maxBlockText and the batch to defaultMaxState, and
// the goal is cut below — it is the anchor's own text, so a session whose first
// message is a pasted file would otherwise put more into the request than every
// block combined, which is the byte cap's whole purpose. One more block's worth is
// the ceiling, because that is what the goal is: one block of text, and the one
// the pinned task lives in.
func state(goal string, blocks []Block) string {
	var b strings.Builder
	b.WriteString("GOAL:\n")
	b.WriteString(clampText(goal, maxBlockText))
	b.WriteString("\n\nHISTORY BLOCKS:\n")
	for _, block := range blocks {
		b.WriteString("\n[")
		b.WriteString(block.Key)
		b.WriteString("]\n")
		b.WriteString(block.Text)
	}
	return b.String()
}

// questions is one choice question per block, all answered in one call. Adding
// more questions does not cost a second round trip for JEV.
//
// Each question names its own block. Every question is evaluated against the same
// state and in isolation, so a question that does not say which block it is about
// leaves the model answering about "it" — and because the batch is otherwise
// identical it answers the same way for every block, which reads as a confident
// unanimous verdict on seventeen different turns. The block key is the only link
// between a question and its block, so the instruction has to carry it.
func questions(blocks []Block) map[string]jev.Question {
	out := make(map[string]jev.Question, len(blocks))
	for _, block := range blocks {
		out[block.Key] = choiceQuestion(block.Key)
	}
	return out
}

// choiceQuestion spells the three options out as the criteria. The value of JEV
// is that it decides rather than writes, so the criteria are the whole contract
// with it — and they have to be disjoint, because an option that overlaps another
// leaves the model no confident answer to give. "A dead end worth keeping" and "a
// dead end with no lasting value" is one such overlap: the same block satisfies
// both, the distribution flattens, and the calibrated confidence drops under the
// floor so no verdict is ever acted on. Each description below names the shape of
// block that belongs to it and nothing else.
//
// A choice's criteria is an object keyed by option and not a list of sentences.
// The endpoint validates the shape and answers 422 to a list, which is the one
// failure a stub server cannot catch — see
// TestChoiceQuestionCriteriaEncodeAsAnObject.
func choiceQuestion(key string) jev.Question {
	return jev.Question{
		Type: "choice",
		Instructions: "The state holds a coding agent's working history as blocks, each headed by its name in square brackets. " +
			"Decide what the agent should still have of the block named [" + key + "]. " +
			"Judge that block only; the rest of the state is there as the context that tells you whether it is still needed.",
		Criteria: map[string]string{
			optionKeep:   "the agent is still working on this: a failure it has not fixed, the code it is editing right now, or a question it has left open",
			optionLedger: "a fact later steps depend on and must not lose: a decision and the reason for it, a constraint or interface, or a load-bearing path, name or command",
			optionPrune:  "output that is already spent: a file since edited, a check that passed, a search that found nothing, a retried command",
		},
		Options: []string{optionKeep, optionPrune, optionLedger},
	}
}
