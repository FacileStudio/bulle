package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/theme"
	"github.com/FacileStudio/nacelle"
)

func TestTheSpinnerAndTheWaitingPhraseAreOneColouredStatement(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()

	line, _, _ := strings.Cut(m.status(), "\n")
	if strings.HasPrefix(line, visible(line)) {
		t.Fatalf("status = %q, want the whole line coloured rather than left plain", line)
	}
	if opens := strings.Index(line, "\x1b["); opens != 0 {
		t.Errorf("status = %q, want the colour to open before the spinner, not after it", line)
	}
	if inner := strings.Count(line, "\x1b["); inner != 2 {
		t.Errorf("status = %q, want one colour and one reset, not %d sequences", line, inner)
	}
}

// The busy loader line is yellow for the whole flight, waiting and running a
// tool alike — its phase is told in text, not colour — while the ✓ ready
// message keeps its green.
func TestTheBusyLoaderIsYellowInFlightAndReadyStaysGreen(t *testing.T) {
	m := sized()
	m.run.busy = true
	m.run.began = time.Now()
	expect := "\x1b[33"
	for _, say := range []string{"waiting for a response", "running run_command"} {
		if say == "running run_command" {
			m.run.beginTool(nacelle.ToolEvent{ID: "1", Name: "run_command", Input: `{}`}, false)
		}
		line := m.status()
		if colourOf(line) != expect || !strings.Contains(visible(line), say) {
			t.Errorf("busy status %q, want it yellow saying %q", line, say)
		}
	}
	m.run.busy = false
	if colourOf(m.status()) != "\x1b[32" {
		t.Errorf("ready status %q, want the done message green as before", m.status())
	}
}

func colourOf(line string) string {
	if !strings.HasPrefix(line, "\x1b[") {
		return ""
	}
	sequence, _, _ := strings.Cut(line, "m")
	return sequence
}

func TestCostInStatusLine(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100, Cost: 0.0123}

	status := visible(m.status())
	if !strings.Contains(status, "$0.0123") {
		t.Fatalf("status = %q, want the reported cost in it", status)
	}
	if strings.Index(status, "$0.0123") > strings.Index(status, "↑2.6k") {
		t.Errorf("status = %q, want the cost ahead of the counts it summarises", status)
	}

	m.spent = nacelle.Usage{InputTokens: 2600, OutputTokens: 1100}
	if status := visible(m.status()); strings.Contains(status, "$") {
		t.Errorf("status = %q, want no currency figure when the backend reported none", status)
	}
}

func TestTheElapsedTimerMeasuresTheRunAndDisappearsWithIt(t *testing.T) {
	m := bareBanner()
	m.began = time.Now().Add(-41 * time.Minute)
	m.run.began = time.Now().Add(-12 * time.Second)
	m.run.busy = true

	if got := m.status(); !strings.Contains(got, "12s") || strings.Contains(got, "41m") {
		t.Errorf("running status = %q, want 12s and not 41m", got)
	}

	m.run.busy = false
	if got := m.status(); strings.Contains(got, "12s") {
		t.Errorf("idle status = %q, want no timer left standing", got)
	}

	m.run.busy = true
	m.run.began = time.Time{}
	if got := m.ongoing(); got != "" {
		t.Errorf("ongoing with zero stamp = %q, want empty", got)
	}
}

func TestStatusSeparatesInputFromOutputTokens(t *testing.T) {
	m := sized()
	m.spent = nacelle.Usage{
		InputTokens:     2600,
		OutputTokens:    1100,
		CacheReadTokens: 9800,
	}

	got := visible(m.View().Content)
	for _, want := range []string{"↑2.6k", "↓1.1k"} {
		if !strings.Contains(got, want) {
			t.Errorf("status %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "3700 tokens") || strings.Contains(got, "13.5k tokens") {
		t.Errorf("status still shows one merged total: %q", got)
	}
	if strings.Contains(got, "cached") {
		t.Errorf("status still shows cached tokens: %q", got)
	}
}

func TestAskingShowsTheSpinnerBeforeAnythingArrives(t *testing.T) {
	m := sized()
	m.agent = answering(t)
	m.prompt.SetValue("are you there?")
	m.ask()
	defer m.run.cancel()

	if !strings.Contains(visible(m.status()), "waiting ") {
		t.Errorf("status = %q, want it saying so while nothing has arrived yet", visible(m.status()))
	}

	m.consume(result{event: nacelle.Event{Kind: nacelle.KindText, Text: "h"}})
	if !m.run.busy {
		t.Fatal("the run stopped being busy on its first event")
	}
	msg, ok := m.spin.Tick().(spinner.TickMsg)
	if !ok {
		t.Fatal("Tick did not produce a spinner.TickMsg")
	}
	if cmd := m.spun(msg); cmd == nil {
		t.Error("the spinner stopped re-arming after the first event")
	}
}

const long = "This is a deliberately long question, far wider than the window it is " +
	"being typed into, so a prompt that refuses to wrap has nowhere to put it. " +
	"And it keeps going well past the second and third line so the growth " +
	"actually pushes the live region out of the way instead of the prompt " +
	"silently overflowing the reserved rows."

func TestPromptWrappingAndGrowth(t *testing.T) {
	m := sized()
	baseline := m.prompt.Height()
	m.prompt.SetValue(long)

	if handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyUp}); handled {
		t.Error("up was claimed by the client, want it left to the prompt")
	}
	if got := m.prompt.Height(); got <= baseline {
		t.Fatalf("prompt height = %d, want it grown past its baseline %d", got, baseline)
	}
	if !strings.Contains(visible(m.prompt.View()), "long question, far wider than the window") {
		t.Errorf("prompt = %q, want the question actually shown", visible(m.prompt.View()))
	}

	before := m.liveRows
	m.layout(m.windowHeight)
	grew := m.prompt.Height() - baseline
	if got := m.liveRows; got != before-grew {
		t.Errorf("live rows = %d, want %d", got, before-grew)
	}

	m.prompt.SetValue(strings.Repeat("word ", 400))
	if got := m.prompt.Height(); got > promptRows {
		t.Errorf("prompt height = %d, want no more than cap %d", got, promptRows)
	}
	verifyPromptClearedRestoresLiveRows(t, m)
}

func verifyPromptClearedRestoresLiveRows(t *testing.T, m *Model) {
	t.Helper()
	m.prompt.Reset()
	m.layout(m.windowHeight)
	tall := m.liveRows
	m.agent = answering(t)
	m.prompt.SetValue(long)
	m.layout(m.windowHeight)
	m.ask()
	defer m.run.cancel()
	if got := m.liveRows; got != tall {
		t.Errorf("live rows = %d, want %d back after prompt cleared", got, tall)
	}
}

func TestPromptContinuationsAndBounds(t *testing.T) {
	border := promptBorder(theme.Themed(true).Border)
	row := border(textarea.PromptInfo{LineNumber: 0, Focused: true})
	if got := visible(row); got != "▌ " || border(textarea.PromptInfo{LineNumber: 1, Focused: true}) != row {
		t.Errorf("row marker = %q, want the same muted ▌ border on every row", got)
	}
	for _, height := range []int{6, 8, 12, 24} {
		m := sized()
		m.resize(tea.WindowSizeMsg{Width: 60, Height: height})
		m.prompt.SetValue(strings.Repeat("word ", 300))
		m.layout(m.windowHeight)
		if got := strings.Count(m.View().Content, "\n") + 1; got > height {
			t.Errorf("window %d: View drew %d rows, want <= %d", height, got, height)
		}
	}
}

func TestPromptHistoryNavigation(t *testing.T) {
	m := sized()
	if handled, _ := m.key(tea.KeyPressMsg{Code: tea.KeyUp}); handled {
		t.Error("up was claimed with an empty history")
	}

	m.hist.Remember("first question", m.Items())
	m.hist.Remember("second question", m.Items())
	m.hist.Remember("first question", m.Items())

	if got := strings.Join(m.hist.Past, "|"); got != "second question|first question" {
		t.Fatalf("history = %q, want duplicate moved to end", got)
	}

	for _, want := range []string{"first question", "second question"} {
		m.key(tea.KeyPressMsg{Code: tea.KeyUp})
		if got := m.prompt.Value(); got != want {
			t.Fatalf("prompt = %q, want %q", got, want)
		}
	}

	m2 := sized()
	m2.hist.Remember("an earlier question", m2.Items())
	m2.prompt.SetValue("half a thought")
	m2.key(tea.KeyPressMsg{Code: tea.KeyUp})
	m2.key(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m2.prompt.Value(); got != "half a thought" {
		t.Fatalf("prompt = %q, want draft restored", got)
	}
}
