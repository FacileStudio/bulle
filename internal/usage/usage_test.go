package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

var sinkNow = time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)

func splitLines(s string) []string {
	trimmed := strings.TrimRight(s, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestARecordedTurnIsAnAssistantMessageMyceliumWillCount(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	sink := NewSink(t.TempDir(), "claude-opus-5")
	if sink == nil {
		t.Fatal("a present data dir must produce a sink")
	}
	sink.Record(nacelle.Usage{
		InputTokens: 100, OutputTokens: 50,
		CacheReadTokens: 7, CacheCreationTokens: 3, Cost: 0.012,
	}, sinkNow)

	path := filepath.Join(dataDir, "events", "bulle", "2026-08.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("mycelium reads events/<agent>/<month>.jsonl: %v", err)
	}
	var got canonicalEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("the line is not JSON: %v", err)
	}
	if got.Type != "message" || got.Role != "assistant" {
		t.Errorf("type/role = %q/%q, want message/assistant", got.Type, got.Role)
	}
	if got.Agent != "bulle" {
		t.Errorf("agent = %q, want bulle so the dashboard labels it", got.Agent)
	}
	checkTurnAccounting(t, got)
}

func checkTurnAccounting(t *testing.T, got canonicalEvent) {
	t.Helper()
	if got.Usage.Input != 100 || got.Usage.Output != 50 ||
		got.Usage.CacheRead != 7 || got.Usage.CacheWrite != 3 {
		t.Errorf("usage = %+v, want the turn's own counts", got.Usage)
	}
	if got.Usage.Cost == nil || got.Usage.Cost.Total != 0.012 {
		t.Errorf("cost = %+v, want the backend's own number", got.Usage.Cost)
	}
	if got.Timestamp != "2026-08-24T10:00:00Z" {
		t.Errorf("timestamp = %q, want RFC3339 UTC", got.Timestamp)
	}
	if got.Model != "claude-opus-5" {
		t.Errorf("model = %q, want the configured one", got.Model)
	}
}

func TestEachTurnAppendsItsOwnLine(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	sink := NewSink(t.TempDir(), "m")

	sink.Record(nacelle.Usage{OutputTokens: 40}, sinkNow)
	sink.Record(nacelle.Usage{OutputTokens: 60}, sinkNow.Add(time.Minute))

	data, err := os.ReadFile(filepath.Join(dataDir, "events", "bulle", "2026-08.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var outputs []int64
	for _, line := range splitLines(string(data)) {
		var ev canonicalEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %q is not JSON: %v", line, err)
		}
		outputs = append(outputs, ev.Usage.Output)
	}
	if len(outputs) != 2 || outputs[0] != 40 || outputs[1] != 60 {
		t.Fatalf("outputs = %v, want each turn once in order", outputs)
	}
}

func TestNoMyceliumMeansNoSinkAndNoStrayTree(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "not-installed")
	t.Setenv("DATA_DIR", absent)

	sink := NewSink(t.TempDir(), "m")
	if sink != nil {
		t.Fatal("an absent data dir must not produce a sink")
	}
	sink.Record(nacelle.Usage{OutputTokens: 1}, sinkNow)
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("a nil sink created %s", absent)
	}
}

func TestAnUnpricedTurnCarriesNoCost(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	sink := NewSink(t.TempDir(), "m")

	sink.Record(nacelle.Usage{InputTokens: 10, OutputTokens: 5}, sinkNow)

	data, err := os.ReadFile(filepath.Join(dataDir, "events", "bulle", "2026-08.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got canonicalEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Usage.Cost != nil {
		t.Fatalf("cost = %+v, want it absent", got.Usage.Cost)
	}
}

func TestRepoNameFromRemote(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:FacileStudio/Mycelium.git": "Mycelium",
		"https://github.com/FacileStudio/nacelle":  "nacelle",
		"https://github.com/FacileStudio/x.git/":   "x",
		"":                                         "",
		"/tmp/a repo":                              "",
	} {
		if got := repoNameFromRemote(url); got != want {
			t.Errorf("repoNameFromRemote(%q) = %q, want %q", url, got, want)
		}
	}
}
