package thinking_test

import (
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/thinking"
)

func TestTheClockStopsWhenTheAnswerStartsNotWhenTheTurnEnds(t *testing.T) {
	var th thinking.Thoughts
	th.Stamp()
	th.Thought()
	spent := th.Elapsed()

	time.Sleep(50 * time.Millisecond)
	if after := th.Elapsed(); after != spent {
		t.Errorf("the clock kept running after the answer began: %s then %s", spent, after)
	}
}

func TestAToolCallStopsTheClockTheSameWay(t *testing.T) {
	var th thinking.Thoughts
	th.Stamp()
	th.Thought()
	spent := th.Elapsed()

	time.Sleep(50 * time.Millisecond)
	if after := th.Elapsed(); after != spent {
		t.Errorf("the tool's own runtime is being billed to thinking: %s then %s", spent, after)
	}
}

func TestForgetDropsRetained(t *testing.T) {
	th := thinking.Thoughts{
		Begun:    time.Now(),
		Ended:    time.Now(),
		Retained: "from the old session",
	}
	th.Forget()
	if th.Retained != "" || !th.Begun.IsZero() || !th.Ended.IsZero() {
		t.Errorf("forget left thoughts dirty: %+v", th)
	}
}

func TestRoughlyFormatsDurations(t *testing.T) {
	cases := []struct {
		spent time.Duration
		want  string
	}{
		{450 * time.Millisecond, "0.5s"},
		{4200 * time.Millisecond, "4.2s"},
		{9949 * time.Millisecond, "9.9s"},
		{10 * time.Second, "10s"},
		{92500 * time.Millisecond, "93s"},
	}
	for _, c := range cases {
		if got := thinking.Roughly(c.spent); got != c.want {
			t.Errorf("Roughly(%s) = %q, want %q", c.spent, got, c.want)
		}
	}
}

func TestCollapsedFormatting(t *testing.T) {
	got := thinking.Collapsed(1500*time.Millisecond, false)
	if got != "\n▶ thought for 1.5s · ctrl+t to expand" {
		t.Errorf("Collapsed with hint = %q", got)
	}
	gotNoHint := thinking.Collapsed(1500*time.Millisecond, true)
	if gotNoHint != "\n▶ thought for 1.5s" {
		t.Errorf("Collapsed without hint = %q", gotNoHint)
	}
	gotZero := thinking.Collapsed(0, false)
	if gotZero != "\n▶ thought · ctrl+t to expand" {
		t.Errorf("Collapsed zero = %q", gotZero)
	}
}
