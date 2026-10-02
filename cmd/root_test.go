package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestRootHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Usage")) {
		t.Errorf("expected Usage section in help output, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("Commands")) {
		t.Errorf("expected Commands section in help output, got: %s", buf.String())
	}
}

func TestCronHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"cron", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on cron help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("install")) {
		t.Errorf("expected install subcommand in cron help, got: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("uninstall")) {
		t.Errorf("expected uninstall subcommand in cron help, got: %s", buf.String())
	}
}

func TestBenchHelpCommand(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"bench", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on bench help, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("--runs")) {
		t.Errorf("expected --runs flag in bench help, got: %s", buf.String())
	}
}

func TestNormalizeArgs(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	os.Args = []string{"bulle", "-model", "gpt-4", "-h", "--root", ".", "--", "-not-a-flag"}
	normalizeArgs()
	if os.Args[1] != "--model" {
		t.Errorf("expected -model normalized to --model, got %s", os.Args[1])
	}
	if os.Args[3] != "-h" {
		t.Errorf("expected short flag -h preserved, got %s", os.Args[3])
	}
	if os.Args[4] != "--root" {
		t.Errorf("expected --root preserved, got %s", os.Args[4])
	}
	if os.Args[7] != "-not-a-flag" {
		t.Errorf("expected arg after -- untouched, got %s", os.Args[7])
	}
}

func TestRootSearchFlags(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	if cmd.Flags().Lookup("search-content") == nil {
		t.Error("search-content flag missing from root command")
	}
	if cmd.Flags().Lookup("find-files") == nil {
		t.Error("find-files flag missing from root command")
	}
}

// The prompt settings are the pair that must not drift apart: system_prompt
// replaces the base prompt, additional_prompt is appended on top of whatever
// is active. Both have to reach the command line.
func TestRootAdditionalPromptFlag(t *testing.T) {
	cmd := newRootCmd("v0.68.0")
	if cmd.Flags().Lookup("additional-prompt") == nil {
		t.Fatal("additional-prompt flag missing from root command")
	}
	if cmd.Flags().Lookup("system-prompt") == nil {
		t.Fatal("system-prompt flag missing from root command")
	}
	if err := cmd.ParseFlags([]string{"--additional-prompt", "stay in character"}); err != nil {
		t.Fatalf("parsing --additional-prompt: %v", err)
	}
	got, err := cmd.Flags().GetString("additional-prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "stay in character" {
		t.Errorf("additional-prompt = %q, want the typed value", got)
	}
}

func TestRootConcurrencyFlags(t *testing.T) {
	cmd := newRootCmd("v0.57.0")
	if cmd.Flags().Lookup("max-concurrency") == nil {
		t.Error("max-concurrency flag missing from root command")
	}
	if cmd.Flags().Lookup("max-parallel-agents") == nil {
		t.Error("max-parallel-agents flag missing from root command")
	}
}
