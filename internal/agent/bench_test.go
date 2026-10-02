package agent

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

func TestBenchTotals(t *testing.T) {
	totals := newBenchTotals(benchFixtureRuns())
	if totals.runs != 2 || totals.duration != 5*time.Second {
		t.Errorf("totals runs/duration = %d/%v, want 2/5s", totals.runs, totals.duration)
	}
	if totals.usage.InputTokens != 300 || totals.usage.OutputTokens != 200 ||
		totals.usage.CacheReadTokens != 600 || totals.usage.CacheCreationTokens != 60 {
		t.Errorf("totals usage = %+v, want 300/200/600/60", totals.usage)
	}
	if math.Abs(totals.usage.Cost-0.04) > 1e-9 {
		t.Errorf("totals cost = %v, want 0.04", totals.usage.Cost)
	}
	if totals.toolCalls != 10 || totals.finalContextTokens != 960 {
		t.Errorf("totals tools/context = %d/%d, want 10/960", totals.toolCalls, totals.finalContextTokens)
	}
}

func TestBenchAverage(t *testing.T) {
	avg := newBenchTotals(benchFixtureRuns()).average()
	if avg.duration != 2500*time.Millisecond {
		t.Errorf("average duration = %v, want 2.5s", avg.duration)
	}
	if avg.usage.InputTokens != 150 || avg.usage.OutputTokens != 100 ||
		avg.usage.CacheReadTokens != 300 || avg.usage.CacheCreationTokens != 30 {
		t.Errorf("average usage = %+v, want 150/100/300/30", avg.usage)
	}
	if math.Abs(avg.usage.Cost-0.02) > 1e-9 {
		t.Errorf("average cost = %v, want 0.02", avg.usage.Cost)
	}
	if avg.toolCalls != 5 || avg.finalContextTokens != 480 {
		t.Errorf("average tools/context = %d/%d, want 5/480", avg.toolCalls, avg.finalContextTokens)
	}
	empty := newBenchTotals(nil).average()
	if empty.runs != 0 {
		t.Errorf("average of no runs = %+v, want zero totals", empty)
	}
}

// benchFixtureRuns builds synthetic measurements: two clean runs and one that
// failed partway, no backend involved.
func benchFixtureRuns() []benchRun {
	return []benchRun{
		{Run: 1, Duration: 2 * time.Second, Stats: runStats{Usage: nacelle.Usage{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 200, CacheCreationTokens: 20, Cost: 0.01}, ToolCalls: 4, FinalContextTokens: 320}},
		{Run: 2, Duration: 3 * time.Second, Stats: runStats{Usage: nacelle.Usage{InputTokens: 200, OutputTokens: 150, CacheReadTokens: 400, CacheCreationTokens: 40, Cost: 0.03}, ToolCalls: 6, FinalContextTokens: 640}},
		{Run: 3, Duration: time.Second, Err: errors.New("boom")},
	}
}

func TestBenchTextTable(t *testing.T) {
	runs := []benchRun{
		{Run: 1, Duration: 2 * time.Second, Stats: runStats{Usage: nacelle.Usage{InputTokens: 10, OutputTokens: 5, CacheReadTokens: 2, CacheCreationTokens: 1, Cost: 0.01}, ToolCalls: 4, FinalContextTokens: 320}},
		{Run: 2, Duration: time.Second, Err: errors.New("boom")},
	}
	want := `  run  duration  input  output  cache     cost  tools  context  status
    1        2s     10       5      3  $0.0100      4      320  ok
    2        1s      0       0      0  $0.0000      0        0  failed
total        2s     10       5      3  $0.0100      4      320
  avg        2s     10       5      3  $0.0100      4      320
run 2 failed: boom
`

	if got := benchTextTable(runs); got != want {
		t.Errorf("benchTextTable() =\n%q\nwant\n%q", got, want)
	}
	clean := []benchRun{runs[0]}
	if notes := benchFailureNotes(clean); notes != "" {
		t.Errorf("benchFailureNotes(clean) = %q, want empty", notes)
	}
}

func TestBenchJSONReport(t *testing.T) {
	data, err := benchJSON(benchFixtureRuns(), "test-model")
	if err != nil {
		t.Fatalf("benchJSON: %v", err)
	}
	var doc benchReport
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the document is not the bench report: %v", err)
	}
	if doc.Model != "test-model" || doc.Failed != 1 || len(doc.Runs) != 3 {
		t.Errorf("report model/failed/runs = %q/%d/%d, want test-model/1/3", doc.Model, doc.Failed, len(doc.Runs))
	}
	last := doc.Runs[2]
	if last.Status != benchFailed || last.Error != "boom" || last.Run != 3 {
		t.Errorf("run 3 record = %+v, want a failed record naming boom", last)
	}
	if doc.Total.InputTokens != 300 || doc.Total.DurationMs != 5000 {
		t.Errorf("total = %+v, want 300 input tokens over 5000ms", doc.Total)
	}
	if doc.Average.ToolCalls != 5 || doc.Average.DurationMs != 2500 {
		t.Errorf("average = %+v, want 5 tool calls over 2500ms", doc.Average)
	}
	if math.Abs(doc.Total.Cost-0.04) > 1e-9 {
		t.Errorf("total cost = %v, want 0.04", doc.Total.Cost)
	}
}

func TestParseBenchFlags(t *testing.T) {
	cases := []struct {
		name    string
		sub     []string
		want    benchFlags
		wantErr bool
	}{
		{"bare prompt", []string{"hello", "world"}, benchFlags{prompt: "hello world", runs: 1}, false},
		{"space form", []string{"-n", "3", "list", "files"}, benchFlags{prompt: "list files", runs: 3}, false},
		{"double dash equals", []string{"--n=2", "x"}, benchFlags{prompt: "x", runs: 2}, false},
		{"json", []string{"-json", "hi"}, benchFlags{prompt: "hi", runs: 1, json: true}, false},
		{"json alone", []string{"--json"}, benchFlags{runs: 1, json: true}, false},
		{"zero runs", []string{"-n", "0", "x"}, benchFlags{}, true},
		{"non-numeric runs", []string{"-n", "abc"}, benchFlags{}, true},
		{"runs without value", []string{"-n"}, benchFlags{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBenchFlags(tc.sub)
			if tc.wantErr {
				benchWantUsageError(t, tc.sub, err)
				return
			}
			benchWantFlags(t, tc.sub, got, err, tc.want)
		})
	}
}

// benchWantUsageError asserts that err is the CLI's usage error.
func benchWantUsageError(t *testing.T, sub []string, err error) {
	t.Helper()
	if _, ok := errors.AsType[*UsageError](err); !ok {
		t.Errorf("parseBenchFlags(%v) = %v; want a usage error", sub, err)
	}
}

// benchWantFlags asserts one parsed flag set against what it should be.
func benchWantFlags(t *testing.T, sub []string, got benchFlags, err error, want benchFlags) {
	t.Helper()
	if err != nil {
		t.Errorf("parseBenchFlags(%v) = %+v, %v; want %+v", sub, got, err, want)
		return
	}
	if got != want {
		t.Errorf("parseBenchFlags(%v) = %+v, want %+v", sub, got, want)
	}
}

func TestBenchDispatch(t *testing.T) {
	savedArgs, savedStdin := os.Args, os.Stdin
	defer func() { os.Args, os.Stdin = savedArgs, savedStdin }()
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("opening the null device: %v", err)
	}
	defer func() { _ = null.Close() }()

	os.Args = []string{"bulle", "cron", "list"}
	if handled, err := checkBenchFlag(); handled || err != nil {
		t.Errorf("checkBenchFlag for cron args = %t, %v; want false, nil", handled, err)
	}

	os.Args = []string{"bulle", "bench"}
	os.Stdin = null
	handled, err := checkBenchFlag()
	if !handled {
		t.Fatal("checkBenchFlag without a prompt = false, want it handled")
	}
	if _, ok := errors.AsType[*UsageError](err); !ok {
		t.Errorf("checkBenchFlag without a prompt = %v; want a usage error", err)
	}

	os.Args = []string{"bulle", "bench", "help"}
	if handled, err := checkBenchFlag(); !handled || err != nil {
		t.Errorf("checkBenchFlag help = %t, %v; want true, nil", handled, err)
	}
}
