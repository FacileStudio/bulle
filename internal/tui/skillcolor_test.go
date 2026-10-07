package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/FacileStudio/bulle/internal/layout"
	"github.com/FacileStudio/bulle/internal/skills"
)

func skillTestModel(skillsList ...string) *Model {
	m := &Model{}
	m.skills = map[string]skills.Skill{}
	for _, name := range skillsList {
		m.skills[name] = skills.Skill{Name: name}
	}
	return m
}

func TestSkillTokens(t *testing.T) {
	m := skillTestModel("review", "plan")
	tokens := m.skillTokens()
	if len(tokens) != 2 || !slices.Contains(tokens, "/skill:plan") || !slices.Contains(tokens, "/skill:review") {
		t.Errorf("skillTokens = %v, want the two /skill names", tokens)
	}
}

func TestHighlightSkillsExact(t *testing.T) {
	row := "use /skill:review on this"
	got := highlightSkills(row, []string{"/skill:review"})
	want := "use \x1b[1;33m/skill:review\x1b[22;39m on this"
	if got != want {
		t.Errorf("highlightSkills = %q, want %q", got, want)
	}
}

func TestHighlightSkillsMultiple(t *testing.T) {
	row := "/skill:review then /skill:plan"
	got := highlightSkills(row, []string{"/skill:plan", "/skill:review"})
	if strings.Count(got, "\x1b[1;33m") != 2 {
		t.Errorf("highlightSkills = %q, want two highlighted tokens", got)
	}
}

func TestHighlightSkillsPartial(t *testing.T) {
	row := "run /skill:rev"
	got := highlightSkills(row, []string{"/skill:review"})
	want := "run \x1b[1;33m/skill:rev\x1b[22;39m"
	if got != want {
		t.Errorf("highlightSkills = %q, want %q", got, want)
	}
}

func TestHighlightSkillsIgnoresUnknown(t *testing.T) {
	row := "/clear and /skillx stay plain"
	got := highlightSkills(row, []string{"/skill:review"})
	if strings.Contains(got, "\x1b[1;33m") {
		t.Errorf("highlightSkills = %q, want no highlight", got)
	}
}

func TestHighlightSkillsRestoresSgr(t *testing.T) {
	row := "\x1b[44m/skill:review rest"
	got := highlightSkills(row, []string{"/skill:review"})
	want := "\x1b[44m\x1b[1;33m/skill:review\x1b[22;39m\x1b[44m rest"
	if got != want {
		t.Errorf("highlightSkills = %q, want %q", got, want)
	}
}

func TestQuestionHighlightsSkills(t *testing.T) {
	m := skillTestModel("review")
	got := m.question("look at /skill:review now", 80)
	if !strings.Contains(got, "\x1b[1;33m/skill:review\x1b[22;39m") {
		t.Errorf("question = %q, want the token highlighted", got)
	}
	if !strings.HasPrefix(layout.Unstyled(got), "▌") {
		t.Errorf("question = %q, want the spine intact", got)
	}
}

func TestPromptViewHighlightsSkills(t *testing.T) {
	m := skillTestModel("review")
	m.skills["review"] = skills.Skill{Name: "review", Description: "d"}
	m.prompt = newPrompt("", lipgloss.NewStyle())
	m.prompt.SetValue("use /skill:review")
	got := highlightSkills(m.prompt.View(), m.skillTokens())
	if !strings.Contains(got, "\x1b[1;33m/skill:review\x1b[22;39m") {
		t.Errorf("prompt view = %q, want the token highlighted", got)
	}
}
