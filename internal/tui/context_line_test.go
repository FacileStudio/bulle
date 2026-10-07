package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/nacelle"
)

// stubJudge is a Judge with no answers: the tests that only care whether a judge
// is attached need a non-nil implementation, not a classification.
type stubJudge struct{}

func (stubJudge) Classify(context.Context, string, []compaction.Block) ([]compaction.Verdict, error) {
	return nil, nil
}

// reportingJudge is a stubJudge that also reports an answer, the shape a judge
// backed by a versioned remote service has.
type reportingJudge struct {
	stubJudge
	answer compaction.Answer
}

func (j reportingJudge) LastAnswer() compaction.Answer { return j.answer }

// The footer names the context against the window it is measured on, with the
// ratio and the tier the size has reached, so a reader can see the ladder
// coming rather than only learning about it from the compaction report.
func TestTheFooterShowsTheContextRatioAndTier(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.size = 120_000

	foot := visible(strings.Join(m.footer(), " "))
	if !strings.Contains(foot, "↕120k/200k · 0.60") {
		t.Errorf("footer = %q, want the size against the window with its ratio", foot)
	}
	if !strings.Contains(foot, "soft") {
		t.Errorf("footer = %q, want the tier the size has reached", foot)
	}
}

// With no window to measure against the ratio is undefined, so the footer keeps
// the plain size and takes the tier from the absolute ceiling instead.
func TestTheFooterKeepsThePlainSizeWithoutAWindow(t *testing.T) {
	m := sized()
	m.engine().Policy = compaction.Policy{
		Ratios:  compaction.Ratios{Soft: 0.65, Smart: 0.80},
		Ceiling: 100_000,
	}
	m.size = 150_000

	foot := visible(strings.Join(m.footer(), " "))
	if !strings.Contains(foot, "↕150k") || strings.Contains(foot, "/") {
		t.Errorf("footer = %q, want the plain size and no ratio", foot)
	}
	if !strings.Contains(foot, "smart") {
		t.Errorf("footer = %q, want the tier the ceiling bought", foot)
	}
}

// /status carries the ladder beyond the footer's figure: the accumulated ledger
// with the tier of the last pass that wrote it.
func TestStatusReportsTheLedgerAndTheLastPassTier(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.size = 150_000
	m.conversation = []nacelle.Message{
		nacelle.UserText("the original task"),
		compaction.BuildLedger("", "a decision, a constraint"),
		nacelle.AssistantText("older work"),
		nacelle.UserText("older answer"),
		nacelle.AssistantText("active turn"),
		nacelle.UserText("the newest turn"),
	}
	m.last = compacted{tier: compaction.Smart, turns: 2}

	m.statusCmd()
	got := strings.Join(m.unprinted, "\n")
	for _, want := range []string{"↕150k/200k · 0.75", "ledger · ~", "last pass smart"} {
		if !strings.Contains(got, want) {
			t.Errorf("status missing %q in %q", want, got)
		}
	}
}

// The denominator the footer prints is the window a turn can actually fill, and
// /status spells out the raw window it came from with the reserve held back for
// the answer. Showing the raw window in the footer would print a ratio under
// smart_ratio while the session was already compacting at smart — the same kind of
// disagreement the tier suffix exists to prevent.
func TestStatusNamesTheReserveTheLadderHoldsBack(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.engine().Policy.Reserve = 40_000
	m.size = 120_000

	m.statusCmd()

	got := strings.Join(m.unprinted, "\n")
	for _, want := range []string{
		"↕120k/160k · 0.75",
		"window · 200k raw, 160k usable, 40.0k reserved for the answer",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("status missing %q in %q", want, got)
		}
	}
}

// A session that has measured nothing and built no ledger says nothing about
// compaction, rather than printing a zero line every time /status is asked.
func TestStatusSaysNothingAboutCompactionBeforeTheFirstPass(t *testing.T) {
	m := sized()
	m.statusCmd()
	got := strings.Join(m.unprinted, "\n")
	for _, want := range []string{"ledger ·", "judge ·", "window ·"} {
		if strings.Contains(got, want) {
			t.Errorf("status = %q, want no %q line with nothing to report", got, want)
		}
	}
}

// windowedAt is the policy the trigger tests measure against: soft at 104k of a
// 160k usable window, with compact_at pinned wherever a case puts it.
func windowedAt(ceiling int64) compaction.Policy {
	return compaction.Policy{
		Ratios:         compaction.Ratios{Soft: 0.65, Smart: 0.80},
		Window:         200_000,
		Reserve:        40_000,
		Ceiling:        ceiling,
		KeepTurns:      3,
		AnchorMessages: 1,
	}
}

// The footer can read a share below soft_ratio under a `soft` suffix, because a
// compact_at pinned under the ratio floors the tier at soft however small the
// share is. /status names the figure the tier is read against, against the same
// denominator the footer uses, so a footer of 0.38 over a trigger at 0.31
// explains itself.
func TestStatusNamesTheCompactAtTheTierIsReadAgainst(t *testing.T) {
	m := sized()
	m.engine().Policy, m.compactAt, m.size = windowedAt(50_000), 50_000, 60_000

	if foot := visible(strings.Join(m.footer(), " ")); !strings.Contains(foot, "↕60.0k/160k · 0.38 · soft") {
		t.Fatalf("footer = %q, want the 0.38 share under a soft suffix, which is the disagreement /status has to explain", foot)
	}

	m.statusCmd()

	if got := strings.Join(m.unprinted, "\n"); !strings.Contains(got, "compact at · 50.0k · 0.31 of 160k usable") {
		t.Errorf("status = %q, want the compact_at the footer's tier is read against", got)
	}
}

// /status names the trigger in every shape it arrives in: pinned, derived from the
// soft ratio of the window, alone when there is no window for either figure to be
// a share of, and not at all when compact_at turned compaction off — where the
// ratio's own figure is a threshold nothing obeys.
func TestStatusNamesTheCompactAtInEveryShapeItComesIn(t *testing.T) {
	tests := []struct {
		name   string
		policy compaction.Policy
		at     int64
		want   string
	}{
		{"pinned under the soft ratio", windowedAt(50_000), 50_000, "compact at · 50.0k · 0.31 of 160k usable"},
		{"derived from the soft ratio", windowedAt(104_000), 104_000, "compact at · 104k · 0.65 of 160k usable"},
		{"with no window", compaction.Policy{Ratios: compaction.Ratios{Soft: 0.65}, Ceiling: 100_000}, 100_000, "compact at · 100k"},
		{"turned off", windowedAt(100_000), 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := sized()
			m.engine().Policy, m.compactAt, m.size = tc.policy, tc.at, 60_000

			m.statusCmd()

			if got := strings.Join(m.unprinted, "\n"); !namesTrigger(got, tc.want) {
				t.Errorf("status = %q, want %q", got, tc.want)
			}
		})
	}
}

// namesTrigger reports whether /status carries the trigger line a case wants, with
// an empty want meaning the line must not be there at all.
func namesTrigger(status, want string) bool {
	if want == "" {
		return !strings.Contains(status, "compact at")
	}
	return strings.Contains(status, want)
}

// The judge is the one setting that sends the conversation off the machine, and
// once enabled nothing else in the UI says so — a reader who opted in months ago
// has nothing to remind them that every pass ships the history somewhere. /status
// is where they already look for the ladder, so the exposure is stated there.
func TestStatusNamesAnOptedInJudge(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.engine().Judge = stubJudge{}
	m.size = 150_000

	m.statusCmd()

	if got := strings.Join(m.unprinted, "\n"); !strings.Contains(got, "judge · on") {
		t.Errorf("status = %q, want the judge named while it is on", got)
	}
}

// The version that answered is named, because the configured model defaults to
// the vendor's drifting alias and the thresholds are tuned against one build
// behind it. Without this line there is nothing to pin the setting to.
func TestStatusNamesTheModelThatAnsweredAndWhatItBilled(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.size = 150_000
	m.engine().Judge = reportingJudge{answer: compaction.Answer{Model: "jev-1.13.0", InputTokens: 12_400, CostUSD: 0.0012}}

	m.statusCmd()

	got := strings.Join(m.unprinted, "\n")
	for _, want := range []string{"judge · on", "judge model · jev-1.13.0", "12.4k tokens in", "$0.0012"} {
		if !strings.Contains(got, want) {
			t.Errorf("status missing %q in %q", want, got)
		}
	}
}

// A judge that has not answered yet — or one that decides locally and implements
// no reporter — adds no model line, so the surface never names a version it never
// saw.
func TestStatusSaysNothingAboutTheJudgeModelBeforeAnAnswer(t *testing.T) {
	m := sized()
	m.engine().Policy = windowedPolicy()
	m.size = 150_000
	m.engine().Judge = reportingJudge{}

	m.statusCmd()

	if got := strings.Join(m.unprinted, "\n"); strings.Contains(got, "judge model") {
		t.Errorf("status = %q, want no model line before the judge has answered", got)
	}
}
