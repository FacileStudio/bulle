package agent

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/nacelle"
)

func TestCronRunRecordCarriesStatusAndError(t *testing.T) {
	stats := runStats{
		Usage:              nacelle.Usage{InputTokens: 100, OutputTokens: 20, Cost: 0.5},
		ToolCalls:          3,
		FinalContextTokens: 120,
		Compactions:        2,
	}
	started := time.Unix(0, 0)
	ok, _ := stats.record(settings.CronJob{Name: "job"}, "model-x", started, nil)
	if ok.Status != string(cronOK) || ok.Error != "" || ok.Usage.Cost != 0.5 || ok.Compactions != 2 {
		t.Fatalf("clean run record wrong: %+v", ok)
	}
	bad, status := stats.record(settings.CronJob{Name: "job"}, "model-x", started, errors.New("boom"))
	if status != cronFailed || bad.Status != "failed" || bad.Error != "boom" {
		t.Fatalf("failed run record wrong: %+v", bad)
	}
}

func TestAppendCronRunWritesOneJSONLine(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	rec, _ := (runStats{Usage: nacelle.Usage{OutputTokens: 7}}).record(settings.CronJob{Name: "job"}, "m", time.Unix(0, 0), nil)
	if err := appendCronRun(rec); err != nil {
		t.Fatal(err)
	}
	path, err := cronStatePath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one JSON line, got %d", len(lines))
	}
	var got cronRun
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "job" || got.Usage.OutputTokens != 7 {
		t.Fatalf("round-trip lost data: %+v", got)
	}
}
