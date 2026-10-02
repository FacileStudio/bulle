package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
	"github.com/FacileStudio/nacelle/tools"
)

// shellSandbox keeps a test's commands away from the real home: the test
// process gets fresh HOME and XDG_CONFIG_HOME, so nothing reaches out and
// scaffolds a real ~/.bulle.yml, and the session's own environment is
// built from those same isolated values.
func shellSandbox(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
}

// shellTestTool builds the run_command replacement the way localTools
// wires it: a real stateless runner underneath as the fallback.
func shellTestTool(t *testing.T, dir, binary string, denyElevation bool) nacelle.Tool {
	t.Helper()
	shellSandbox(t)

	set, err := tools.New(tools.Config{Root: dir, AllowBash: true})
	if err != nil {
		t.Fatalf("opening the tool set: %v", err)
	}
	all, err := set.Tools()
	if err != nil {
		t.Fatalf("building the tool set: %v", err)
	}
	var fallback nacelle.Tool
	for _, tool := range all {
		if tool.Name() == "run_command" {
			fallback = tool
		}
	}
	if fallback == nil {
		t.Fatal("the SDK set has no run_command to fall back to")
	}

	session := newShellSession(dir, []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}, denyElevation)
	session.binary = binary
	tool, err := newShellTool(session, fallback)
	if err != nil {
		t.Fatalf("building the shell tool: %v", err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Logf("session teardown: %v", err)
		}
	})
	return tool
}

func shellRun(t *testing.T, tool nacelle.Tool, in shellCommandInput) string {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshalling the input: %v", err)
	}
	out, err := tool.Run(context.Background(), raw)
	if err != nil {
		t.Fatalf("run_command %q: %v", in.Command, err)
	}
	return out
}

func TestShellSessionKeepsCwdAndEnvAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	tool := shellTestTool(t, dir, "bash", false)

	first := shellRun(t, tool, shellCommandInput{Command: "cd sub && export BULLE_SHELL_TEST=hello"})
	if strings.Contains(first, "exit status") {
		t.Fatalf("setting up state failed: %s", first)
	}

	second := shellRun(t, tool, shellCommandInput{Command: "pwd && echo $BULLE_SHELL_TEST"})
	if want := sub + "\nhello"; second != want {
		t.Fatalf("state did not survive the call boundary:\n got %q\nwant %q", second, want)
	}
}

func TestShellSessionReportsExitStatus(t *testing.T) {
	tool := shellTestTool(t, t.TempDir(), "bash", false)

	got := shellRun(t, tool, shellCommandInput{Command: "echo out; false"})
	if want := "out\n\n[exit status 1]"; got != want {
		t.Fatalf("wrong result shape:\n got %q\nwant %q", got, want)
	}
}

func TestShellSessionSurvivesExitAndRespawns(t *testing.T) {
	dir := t.TempDir()
	tool := shellTestTool(t, dir, "bash", false)

	if got := shellRun(t, tool, shellCommandInput{Command: "exit 3"}); !strings.Contains(got, "[exit status 3]") {
		t.Fatalf("the shell's own exit status is missing: %q", got)
	}
	if got := shellRun(t, tool, shellCommandInput{Command: "pwd"}); got != dir {
		t.Fatalf("the replacement shell did not start in the root: %q", got)
	}
}

func TestShellToolFallsBackWhenBashIsMissing(t *testing.T) {
	dir := t.TempDir()
	tool := shellTestTool(t, dir, "/nonexistent/bulle-missing-bash", false)

	if got := shellRun(t, tool, shellCommandInput{Command: "echo fallen-back"}); got != "fallen-back" {
		t.Fatalf("the stateless fallback did not answer: %q", got)
	}
	if got := shellRun(t, tool, shellCommandInput{Command: "echo still-falling-back"}); got != "still-falling-back" {
		t.Fatalf("the fallback is not sticky: %q", got)
	}
}

func TestShellSessionTimesOutAndRecovers(t *testing.T) {
	tool := shellTestTool(t, t.TempDir(), "bash", false)

	got := shellRun(t, tool, shellCommandInput{Command: "sleep 30", Timeout: 1})
	if !strings.Contains(got, "timed out after 1s") || !strings.Contains(got, "the command and its children were killed") {
		t.Fatalf("the timeout is not reported the way the stateless runner reports it: %q", got)
	}
	if got := shellRun(t, tool, shellCommandInput{Command: "echo recovered"}); got != "recovered" {
		t.Fatalf("no shell after the timeout: %q", got)
	}
}

func TestShellToolStreamsLinesWithoutTheMarker(t *testing.T) {
	tool := shellTestTool(t, t.TempDir(), "bash", false)
	streamer, ok := tool.(interface {
		RunOutput(context.Context, json.RawMessage, func(string)) (string, error)
	})
	if !ok {
		t.Fatal("the shell tool does not stream output")
	}

	raw, err := json.Marshal(shellCommandInput{Command: "printf 'a\\nb\\nc'"})
	if err != nil {
		t.Fatalf("marshalling the input: %v", err)
	}
	var got []string
	out, err := streamer.RunOutput(context.Background(), raw, func(line string) { got = append(got, line) })
	if err != nil {
		t.Fatalf("RunOutput: %v", err)
	}
	if out != "a\nb\nc" {
		t.Fatalf("the result changed shape: %q", out)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("the streamed lines are wrong: %q", got)
	}
}

func TestLocalToolsMountsThePersistentShell(t *testing.T) {
	shellSandbox(t)
	dir := t.TempDir()
	on, off := true, false
	config := Config{
		Session:  Session{Root: dir},
		Toggles:  Toggles{Bash: &on, Fetch: &off},
		Security: Security{PathIsolation: &off, DenyElevation: &off, EnvIsolation: &off},
	}

	set, local, err := localTools(config)
	if err != nil {
		t.Fatalf("localTools: %v", err)
	}
	defer set.Close()

	mounted := ""
	for _, tool := range local {
		if tool.Name() == "run_command" {
			mounted = tool.Description()
		}
	}
	if !strings.Contains(mounted, "keeps its state between calls") {
		t.Fatalf("the persistent shell is not mounted:\n%s", mounted)
	}
}

func TestLocalToolsKeepsTheStatelessShellUnderStrictConfinement(t *testing.T) {
	shellSandbox(t)
	dir := t.TempDir()
	on, off := true, false
	config := Config{
		Session:  Session{Root: dir},
		Toggles:  Toggles{Bash: &on, Fetch: &off},
		Security: Security{PathIsolation: &on, DenyElevation: &off, EnvIsolation: &off},
	}

	set, local, err := localTools(config)
	if err != nil {
		t.Fatalf("localTools: %v", err)
	}
	defer set.Close()

	mounted := ""
	for _, tool := range local {
		if tool.Name() == "run_command" {
			mounted = tool.Description()
		}
	}
	if mounted == "" || strings.Contains(mounted, "keeps its state between calls") {
		t.Fatalf("strict confinement did not keep the stateless runner:\n%s", mounted)
	}
}
