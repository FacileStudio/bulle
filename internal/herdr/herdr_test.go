package herdr

import (
	"strings"
	"testing"
)

// recording builds a client whose spawn captures every command instead of
// running it, so the tests can assert what would have been sent to herdr.
// calls returns what was captured at the moment it is read, not a snapshot
// taken when recording returned.
func recording() (*Client, func() []string) {
	var calls []string
	h := &Client{
		bin:  "/bin/herdr",
		pane: "w1:p1",
		spawn: func(args []string) error {
			calls = append(calls, strings.Join(args, " "))
			return nil
		},
	}
	return h, func() []string { return calls }
}

func TestNewFromEnvIsNilOutsideHerdr(t *testing.T) {
	t.Setenv("HERDR_ENV", "")
	if NewFromEnv() != nil {
		t.Error("a client was built with HERDR_ENV unset or empty")
	}
}

func TestNewFromEnvIsNilWithoutPaneOrBin(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_BIN_PATH", "")
	t.Setenv("HERDR_PANE_ID", "")
	if NewFromEnv() != nil {
		t.Error("a client was built without a pane id and binary path")
	}
}

func TestNewFromEnvBuildsFromEnv(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_BIN_PATH", "/path/to/herdr")
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	h := NewFromEnv()
	if h == nil {
		t.Fatal("no client was built with the full herdr environment")
	}
	if h.bin != "/path/to/herdr" || h.pane != "w1:p1" {
		t.Errorf("client = bin %q pane %q, want the injected herdr values", h.bin, h.pane)
	}
}

func TestReportDedupsAnUnchangedState(t *testing.T) {
	h, calls := recording()
	Report(h, Working)
	Report(h, Working)
	if len(calls()) != 1 {
		t.Errorf("reporting the same state twice fired %d command(s), want 1", len(calls()))
	}
}

func TestReportSendsWorkingAndTransitions(t *testing.T) {
	h, calls := recording()
	Report(h, Working)
	Report(h, Blocked)
	if len(calls()) != 2 {
		t.Fatalf("two distinct states fired %d command(s), want 2", len(calls()))
	}
	if calls()[0] != "/bin/herdr pane report-agent w1:p1 --source bulle --agent bulle --state working" {
		t.Errorf("working report = %q", calls()[0])
	}
	if calls()[1] != "/bin/herdr pane report-agent w1:p1 --source bulle --agent bulle --state blocked" {
		t.Errorf("blocked report = %q", calls()[1])
	}
}

func TestReportCarriesTheSessionPathOnceSet(t *testing.T) {
	h, calls := recording()
	Report(h, Working)
	if len(calls()) != 1 || strings.Contains(calls()[0], "--agent-session-path") {
		t.Errorf("pre-session report = %+v, want no session reference yet", calls())
	}

	SetSession(h, "/home/me/.bulle/sessions/2026-09-10T14-000Z.jsonl")
	Report(h, Blocked)
	if len(calls()) != 2 {
		t.Fatalf("two distinct states fired %d command(s), want 2", len(calls()))
	}
	expected := "/bin/herdr pane report-agent w1:p1 --source bulle --agent bulle --state blocked " +
		"--agent-session-path /home/me/.bulle/sessions/2026-09-10T14-000Z.jsonl"
	if calls()[1] != expected {
		t.Errorf("session-bearing report = %q\nwant = %q", calls()[1], expected)
	}
}

func TestReleaseSendsReleaseAndIsNilSafe(t *testing.T) {
	h, calls := recording()
	Release(h)
	if len(calls()) != 1 || calls()[0] != "/bin/herdr pane release-agent w1:p1 --source bulle --agent bulle" {
		t.Errorf("release = %+v, want the release command", calls())
	}
	Report(nil, Working)
	Release(nil)
}
