package agent

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

func defaults() Config {
	return settings.Defaults("")
}

func withApproval(on bool) Config {
	config := defaults()
	bash := true
	config.ApproveTools, config.Bash = &on, &bash
	return config
}

func TestEnvironmentDescribesTheTargetLoginDirectoryForARemoteSession(t *testing.T) {
	config := withApproval(false)
	config.Root = "target:staging"
	remote := environment(config, time.Now(), connected{})

	for _, want := range []string{"staging", "SSH login directory", "over SSH"} {
		if !strings.Contains(remote, want) {
			t.Errorf("environment() = %q, want it to tell the model the tools run on %q over SSH", remote, want)
		}
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(remote, "Working directory: "+cwd) {
		t.Errorf("environment() = %q, want no host path presented as the working directory", remote)
	}
	if strings.Contains(remote, "no confinement") {
		t.Errorf("environment() = %q, want no host-confinement wording for a remote session", remote)
	}
}

func TestEnvironmentNamesTheRootAbsolutely(t *testing.T) {
	config := withApproval(false)
	config.Root = "."

	got := environment(config, time.Now(), connected{})

	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, want) {
		t.Errorf("environment() = %q, want the root resolved to %q rather than left as \".\"", got, want)
	}
}

func TestEnvironmentWarnsThatAbsolutePathsOnlyWorkInRunCommand(t *testing.T) {
	got := environment(withApproval(false), time.Now(), connected{})

	for _, want := range []string{"absolute", "run_command"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to mention %q", got, want)
		}
	}
}

func TestEnvironmentDatesTheSession(t *testing.T) {
	got := environment(withApproval(false), time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC), connected{})

	if !strings.Contains(got, "2026-08-20") {
		t.Errorf("environment() = %q, want it to carry the date it was given", got)
	}
}

func TestEnvironmentSaysWhetherAnyoneReviewsAToolCall(t *testing.T) {
	on := environment(withApproval(true), time.Now(), connected{})
	if !strings.Contains(on, "refusal") {
		t.Errorf("environment() = %q, want the approval gate explained when it is on", on)
	}
	if strings.Contains(on, "Nobody sees one first") {
		t.Errorf("environment() = %q, want no unreviewed warning while the gate is on", on)
	}

	if off := environment(withApproval(false), time.Now(), connected{}); !strings.Contains(off, "Nobody sees one first") {
		t.Errorf("environment() = %q, want the model told nothing reviews a call with the gate off", off)
	}
}

func TestEnvironmentTeachesBatchingIndependentToolCalls(t *testing.T) {
	got := environment(withApproval(false), time.Now(), connected{})

	for _, want := range []string{"independent", "one turn"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to teach batching with %q", got, want)
		}
	}
}

func TestEnvironmentSaysAParallelFanOutEndsTheTurn(t *testing.T) {
	got := environment(withApproval(false), time.Now(), connected{})

	for _, want := range []string{"non-blocking and return-control", "ends your turn"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to tell the model a dispatched fan-out is the point to stop, with %q", got, want)
		}
	}
}

func TestEnvironmentWarnsAboutIrreversibleCommandsOnlyWithBash(t *testing.T) {
	if got := environment(withApproval(false), time.Now(), connected{}); !strings.Contains(got, "git reset --hard") {
		t.Errorf("environment() = %q, want the irreversible commands named when the shell is on", got)
	}

	config := withApproval(false)
	off := false
	config.Bash = &off
	if got := environment(config, time.Now(), connected{}); strings.Contains(got, "git reset --hard") {
		t.Errorf("environment() = %q, want no shell warning when there is no shell", got)
	}
}

func TestAugmentSystemAlwaysAppendsTheEnvironment(t *testing.T) {
	off := false
	config := defaults()
	config.System = "You are something else entirely."
	config.ProjectContext, config.Skills = &off, &off

	augmentSystem(&config, connected{})

	if !strings.Contains(config.System, "Working directory:") {
		t.Errorf("System = %q, want the session's own facts appended regardless of the switches", config.System)
	}
}

func TestEnvironmentMentionsRunCommandOnlyWhenBashIsMounted(t *testing.T) {
	config := withApproval(false)
	off := false
	config.Bash = &off

	if got := environment(config, time.Now(), connected{}); strings.Contains(got, "run_command") {
		t.Errorf("environment() = %q, want no word about run_command when -bash is off", got)
	}
}

func TestEnvironmentMentionsWebFetchOnlyWhenFetchIsMounted(t *testing.T) {
	got := environment(withApproval(false), time.Now(), connected{})
	if !strings.Contains(got, "web_fetch") {
		t.Errorf("environment() = %q, want the fetch tool named when fetch is on", got)
	}

	config := withApproval(false)
	off := false
	config.Fetch = &off

	if got := environment(config, time.Now(), connected{}); strings.Contains(got, "web_fetch") {
		t.Errorf("environment() = %q, want no word about web_fetch when fetch is off", got)
	}
}

func TestEnvironmentDescribesConfinementBasedOnPathIsolation(t *testing.T) {

	on := environment(withApproval(false), time.Now(), connected{})
	if !strings.Contains(on, "absolute paths") {
		t.Errorf("environment() without confinement = %q, want mention of absolute-path handling", on)
	}
	if !strings.Contains(on, "no confinement") {
		t.Errorf("environment() without confinement = %q, want \"no confinement\" mentioned", on)
	}

	with := withApproval(false)
	confined := true
	with.PathIsolation = &confined
	withConfined := environment(with, time.Now(), connected{})
	if !strings.Contains(withConfined, "cannot reach outside") {
		t.Errorf("environment() with confinement = %q, want confinement warning", withConfined)
	}
	if strings.Contains(withConfined, "no confinement") {
		t.Errorf("environment() with confinement = %q, should not say \"no confinement\"", withConfined)
	}
}

func TestEnvironmentNamesTheBridgedServersOnlyWhenMounted(t *testing.T) {
	if got := environment(defaults(), time.Now(), connected{}); strings.Contains(got, "MCP-bridged") {
		t.Errorf("environment() = %q, want no MCP note when no server is mounted", got)
	}

	got := environment(defaults(), time.Now(), connected{servers: 2, names: []string{"antenne", "sonde"}})
	for _, want := range []string{"antenne, sonde", "prefixed with"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want it to say %q", got, want)
		}
	}
}

func TestEnvironmentExplainsTheCatalogWhenOneIsMounted(t *testing.T) {
	if got := environment(defaults(), time.Now(), connected{servers: 1, names: []string{"antenne"}}); strings.Contains(got, "search_tools") {
		t.Errorf("environment() = %q, want no catalog note while the set is mounted whole", got)
	}

	got := environment(defaults(), time.Now(), connected{servers: 1, names: []string{"antenne"}, catalog: true})
	for _, want := range []string{"search_tools", "get_tool_details", "call_tool"} {
		if !strings.Contains(got, want) {
			t.Errorf("environment() = %q, want the catalog note to name %s", got, want)
		}
	}
}

func TestEnvironmentMentionsSearchToolsOnlyWhenMounted(t *testing.T) {
	got := environment(defaults(), time.Now(), connected{})
	if !strings.Contains(got, "search_content") {
		t.Errorf("environment() = %q, want search_content named when SearchContent is on", got)
	}
	if !strings.Contains(got, "find_files") {
		t.Errorf("environment() = %q, want find_files named when FindFiles is on", got)
	}

	config := defaults()
	off := false
	config.SearchContent = &off
	config.FindFiles = &off

	disabled := environment(config, time.Now(), connected{})
	if strings.Contains(disabled, "search_content") {
		t.Errorf("environment() = %q, want no search_content when disabled", disabled)
	}
	if strings.Contains(disabled, "find_files") {
		t.Errorf("environment() = %q, want no find_files when disabled", disabled)
	}
}

func TestDefaultSystemPromptDoesNotMentionListDirectory(t *testing.T) {
	prompt := DefaultSystemPrompt()
	if strings.Contains(prompt, "list_directory") {
		t.Errorf("DefaultSystemPrompt() contains phantom tool list_directory")
	}
}
