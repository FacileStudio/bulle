package compaction

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/FacileStudio/nacelle"
)

func TestJudgeGoalIncludesBothPartsWithLabels(t *testing.T) {
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.ToolCall{ID: "c1", Name: "read", Input: json.RawMessage(`{}`), Finished: true}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.ToolResult{ID: "c1", Name: "read", Result: "x"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer one"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "a newer turn"}}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})
	goal := JudgeGoal(conv, plan)

	for _, want := range []string{"Session goal:", "Current request:", "the task", "a newer turn"} {
		if !strings.Contains(goal, want) {
			t.Errorf("goal missing %q, got: %q", want, goal)
		}
	}
}

func TestJudgeGoalShortSessionIsAnchorOnly(t *testing.T) {
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer one"}}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})
	if goal := JudgeGoal(conv, plan); goal != "the task" {
		t.Errorf("JudgeGoal = %q, want byte-identical anchor %q", goal, "the task")
	}
}

func TestJudgeGoalNoUserMessagesAfterAnchor(t *testing.T) {
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.ToolCall{ID: "c1", Name: "read", Input: json.RawMessage(`{}`), Finished: true}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.ToolResult{ID: "c1", Name: "read", Result: "x"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer one"}}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})
	if goal := JudgeGoal(conv, plan); goal != "the task" {
		t.Errorf("goal with no later user text = %q, want the anchor", goal)
	}
}

func TestJudgeGoalCapsDirective(t *testing.T) {
	long := "very long current request " + strings.Repeat("extra", 10000)
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer one"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: long}}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})
	goal := JudgeGoal(conv, plan)

	if !strings.Contains(goal, "very long current request") {
		t.Errorf("goal missing the directive prefix, got: %q", goal[:80])
	}
	if runes := utf8.RuneCountInString(goal); runes > goalRequestRunes+64 {
		t.Errorf("goal is %d runes, want at most goalRequestRunes (%d) plus labels", runes, goalRequestRunes)
	}
}

func TestJudgeGoalNewestDirectiveWins(t *testing.T) {
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.ToolCall{ID: "c1", Name: "read", Input: json.RawMessage(`{}`), Finished: true}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.ToolResult{ID: "c1", Name: "read", Result: "x"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer one"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "a newer turn"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.ToolCall{ID: "c2", Name: "write", Input: json.RawMessage(`{}`), Finished: true}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.ToolResult{ID: "c2", Name: "write", Result: "y"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer two"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "yet another turn"}}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 2})
	goal := JudgeGoal(conv, plan)

	if !strings.Contains(goal, "yet another turn") {
		t.Errorf("goal missing the newest user directive, got: %q", goal)
	}
	for _, unwanted := range []string{"a newer turn", "answer one", "answer two", "\"x\"", "\"y\""} {
		if strings.Contains(goal, unwanted) {
			t.Errorf("goal must not contain %q, got: %q", unwanted, goal)
		}
	}
}

func TestJudgeGoalSkipsToolResultsInsideTurns(t *testing.T) {
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.ToolCall{ID: "c1", Name: "read", Input: json.RawMessage(`{}`), Finished: true}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.ToolResult{ID: "c1", Name: "read", Result: "tool output"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer"}}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})
	if goal := JudgeGoal(conv, plan); goal != "the task" {
		t.Errorf("a tool result must not become the directive, got: %q", goal)
	}
}

func TestJudgeGoalConcatenatesMultipartUserText(t *testing.T) {
	conv := []nacelle.Message{
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{nacelle.Text{Text: "the task"}}},
		{Role: nacelle.RoleAssistant, Parts: []nacelle.Part{nacelle.Text{Text: "answer"}}},
		{Role: nacelle.RoleUser, Parts: []nacelle.Part{
			nacelle.Text{Text: "part one: "},
			nacelle.Text{Text: "part two"},
		}},
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})
	goal := JudgeGoal(conv, plan)
	if !strings.Contains(goal, "part one: part two") {
		t.Errorf("JudgeGoal = %q, want concatenated parts", goal)
	}
}
