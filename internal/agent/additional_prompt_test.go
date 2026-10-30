package agent

import (
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

// The additional prompt is appended last — after the environment block, the
// project context and the skills catalog — so the steering text sits closest
// to the turn, where it carries the most weight over the guidance beneath it.
func TestAugmentSystemAppendsTheAdditionalPromptLast(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())
	config := defaults()
	config.Additional = "You are a terse reviewer."

	augmentSystem(&config, connected{})

	at := strings.LastIndex(config.System, "You are a terse reviewer.")
	if at < 0 {
		t.Fatalf("System = %q, want the additional prompt appended", config.System)
	}
	if tail := strings.TrimSpace(config.System[at:]); tail != "You are a terse reviewer." {
		t.Errorf("text after the additional prompt = %q, want none", tail)
	}
}

// A base prompt replaced via system_prompt is appended to just the same: the
// harness guidance is gone, and the persona layers onto whatever is there.
func TestAugmentSystemLayersTheAdditionalPromptOverAReplacedBase(t *testing.T) {
	xdgtest.Isolate(t, t.TempDir())
	config := defaults()
	config.System = "You are something else entirely."
	config.Additional = "Always answer in French."

	augmentSystem(&config, connected{})

	if !strings.HasPrefix(config.System, "You are something else entirely.") {
		t.Errorf("System = %q, want the replacement base to stay at the front", config.System)
	}
	if !strings.HasSuffix(config.System, "Always answer in French.") {
		t.Errorf("System = %q, want the additional prompt to close the prompt", config.System)
	}
}
