package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/cost"
	"github.com/FacileStudio/bulle/internal/status"
	"github.com/FacileStudio/nacelle"
)

// turn records a finished turn. Its boundary line is no longer said here:
// each intermediate recap drowned the transcript, and the footer already
// ticks the live counts. The one recap the reader gets is the run's, said
// once at settle.
func (m *Model) turn(event nacelle.Event) {
	m.Thought()
	m.flushThinking()
	m.commitTail()
	m.run.usage = m.run.usage.Add(event.Usage)
	m.learnRate(event.Usage)
	m.run.liveOut = 0
	m.sink.Record(event.Usage, time.Now())
	m.sized(event.Usage)
	m.run.turnBegan = time.Time{}
}

func (m *Model) commitTail() {
	m.commitParagraphs()
	if m.run.answer.Len() == 0 {
		return
	}
	tail := m.run.answer.String()
	m.run.committedLen += len(tail)
	m.run.answer.Reset()
	if strings.TrimSpace(tail) != "" {
		m.say(fromModel, tail)
	}
}

// runRecap says the run's one boundary line at settle: the run's duration,
// its total token spend and its cost. The per-turn version is gone; this is
// the one recap that survives.
func (m *Model) runRecap() {
	if m.run.usage.Total() == 0 {
		return
	}
	spent := time.Duration(0)
	if !m.run.began.IsZero() {
		spent = time.Since(m.run.began)
	}
	dur := "0s"
	if spent > 0 {
		dur = status.Took(spent)
	}
	tokens := m.run.usage.Total()
	tokenStr := fmt.Sprintf("%s tokens", status.ShortTokens(tokens))
	if tokens == 1 {
		tokenStr = "1 token"
	}
	pieces := []string{dur, tokenStr}
	if m.run.usage.Cost > 0 {
		pieces = append(pieces, cost.FormatCost(m.run.usage.Cost))
	}
	m.say(fromTurn, strings.Join(pieces, " · "))
}

// learnRate records the realised cost per token of a finished turn, Cost
// divided by the turn's total billed tokens. The status line scales the live
// output-token estimate by it while the next turn streams, so the dollar
// figure moves before the turn ends. Only a reported Cost ever sets it, and a
// turn reporting none leaves the previous rate standing: a backend that never
// reports a Cost keeps the rate at zero, which is what keeps the live price
// invisible there. The authoritative per-turn Cost still lands in the footer
// the moment the turn ends, replacing the estimate exactly as the live token
// estimate is replaced.
func (m *Model) learnRate(usage nacelle.Usage) {
	if usage.Cost <= 0 || usage.Total() == 0 {
		return
	}
	m.rate = usage.Cost / float64(usage.Total())
}

func (m *Model) flushThinking() {
	reasoning := m.run.reasoning.String()
	m.run.reasoning.Reset()

	spent := m.Elapsed()
	if reasoning != "" || m.run.reasoningFull.Len() > 0 {
		m.Begun, m.Ended = time.Time{}, time.Time{}
		m.Retained = m.run.reasoningFull.String() + reasoning
		m.run.reasoningFull.Reset()
	}

	if reasoning != "" {
		if m.Expanded {
			m.say(fromThinking, reasoning)
		} else {
			m.say(fromThinking, m.Collapsed(spent))
		}
	}
}

// introduceReasoning commits pending reasoning to the scrollback, above the
// output about to stream below it. The scrollback is append-only: once the
// answer's paragraphs are printed under it, the line can never move back up, so
// it has to go in before the first paragraph does. That is the whole bug it
// exists for — turn() deferred the line to the end of the turn, by which point
// every completed answer paragraph had already been printed beneath it.
//
// It only introduces reasoning that was not already shown. The text is moved
// into reasoningFull so flushThinking at the end of the turn still folds it
// into Retained, and the reasoning buffer is cleared so streaming() and the
// end-of-turn flush do not draw it a second time.
func (m *Model) introduceReasoning() {
	if m.run.reasoning.Len() == 0 {
		return
	}
	if m.Expanded {
		m.say(fromThinking, m.run.reasoning.String())
	} else {
		m.say(fromThinking, m.Collapsed(m.Elapsed()))
	}
	m.run.reasoningFull.WriteString(m.run.reasoning.String())
	m.run.reasoningFull.WriteString("\n")
	m.run.reasoning.Reset()
}

func (m *Model) flush() string {
	m.flushThinking()

	full := m.run.fullAnswer.String()
	unprinted := ""
	if m.run.committedLen < len(full) {
		unprinted = full[m.run.committedLen:]
	}
	m.run.answer.Reset()
	m.run.fullAnswer.Reset()
	m.run.committedLen = 0
	if strings.TrimSpace(unprinted) != "" {
		m.say(fromModel, unprinted)
	}
	return full
}

func (m *Model) reveal() (bool, tea.Cmd) {
	m.Expanded = !m.Expanded
	m.Hinted = true

	switch {
	case m.Expanded && m.Retained != "":
		m.say(fromThinking, m.Retained)
	case m.Expanded:
		m.say(fromClient, "reasoning will be shown in full from here")
	default:
		m.say(fromClient, "reasoning will collapse to a single line from here")
	}
	return true, nil
}
