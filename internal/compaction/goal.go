package compaction

import (
	"strings"

	"github.com/FacileStudio/nacelle"
)

// goalRequestRunes caps how much of the latest user directive rides along in a
// judge goal: a directive is context, not payload, and the judge state has a
// budget to respect.
const goalRequestRunes = 4000

// JudgeGoal is the task the judge classifies against: the pinned session goal
// plus the most recent user directive, so a session whose task moved on is
// judged against where it stands now. With no directive past the anchor it is
// byte-identical to GoalText, so short sessions judge exactly as before.
func JudgeGoal(conv []nacelle.Message, plan []Span) string {
	anchor := GoalText(conv, plan)
	directive := latestUserDirective(conv, plan)
	if directive == "" {
		return anchor
	}
	if runes := []rune(directive); len(runes) > goalRequestRunes {
		directive = string(runes[:goalRequestRunes]) + "..."
	}
	return "Session goal:\n" + anchor + "\n\nCurrent request:\n" + directive
}

// latestUserDirective is the newest user-authored text after the anchor,
// whichever zone holds it. Tool results and assistant text never qualify: the
// directive is what the person asked for, not what the machinery returned.
func latestUserDirective(conv []nacelle.Message, plan []Span) string {
	for i := len(conv) - 1; i >= anchorZoneEnd(plan); i-- {
		if conv[i].Role != nacelle.RoleUser {
			continue
		}
		if text := userMessageText(conv[i]); text != "" {
			return text
		}
	}
	return ""
}

// userMessageText collects all text parts from one user message.
func userMessageText(m nacelle.Message) string {
	var b strings.Builder
	for _, part := range m.Parts {
		if text, ok := part.(nacelle.Text); ok && text.Text != "" {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

// anchorZoneEnd is where the anchor zone stops: the highest End among anchor spans.
func anchorZoneEnd(plan []Span) int {
	end := 0
	for _, span := range plan {
		if span.Zone == ZoneAnchor && span.End > end {
			end = span.End
		}
	}
	return end
}
