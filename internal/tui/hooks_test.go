package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/nacelle"
)

func TestHookExecutionShownInConversation(t *testing.T) {
	cfg := SessionConfig{
		Hooks: HookUIConfig{Show: true, ShowOutput: true},
	}
	m := NewModel(nil, "banner", nil, cfg)
	m.recordHook(hookReportMsg{
		report: settings.HookReport{
			Event:    nacelle.AfterToolCall,
			Tool:     "edit_file",
			Command:  "graft blast --no-refresh",
			Duration: 15 * time.Millisecond,
			Stdout:   "blast radius: 2 files",
		},
	})

	output := strings.Join(spoken(m), "\n")
	if !strings.Contains(output, "hook:after_tool_call[edit_file]") {
		t.Errorf("got %q, want it to contain hook:after_tool_call[edit_file]", output)
	}
	if !strings.Contains(output, "blast radius: 2 files") {
		t.Errorf("got %q, want it to contain preview output", output)
	}
}

func TestHookExecutionWithoutOutputWhenDisabled(t *testing.T) {
	cfg := SessionConfig{
		Hooks: HookUIConfig{Show: true, ShowOutput: false},
	}
	m := NewModel(nil, "banner", nil, cfg)
	m.recordHook(hookReportMsg{
		report: settings.HookReport{
			Event:    nacelle.AfterToolCall,
			Tool:     "edit_file",
			Command:  "graft blast --no-refresh",
			Duration: 15 * time.Millisecond,
			Stdout:   "blast radius: 2 files",
		},
	})

	output := strings.Join(spoken(m), "\n")
	if !strings.Contains(output, "hook:after_tool_call[edit_file]") {
		t.Errorf("got %q, want hook line", output)
	}
	if strings.Contains(output, "blast radius: 2 files") {
		t.Errorf("got %q, expected no preview output when show_hook_output is false", output)
	}
}

func TestHookExecutionNotShownWhenDisabled(t *testing.T) {
	cfg := SessionConfig{
		Hooks: HookUIConfig{Show: false, ShowOutput: false},
	}
	m := NewModel(nil, "banner", nil, cfg)
	m.recordHook(hookReportMsg{
		report: settings.HookReport{
			Event:    nacelle.AfterToolCall,
			Tool:     "edit_file",
			Command:  "graft blast --no-refresh",
			Duration: 15 * time.Millisecond,
			Stdout:   "blast radius: 2 files",
		},
	})

	if len(spoken(m)) != 0 {
		t.Fatalf("got %d spoken lines, want 0 when show_hooks is false", len(spoken(m)))
	}
}

func TestHookExecutionWithLabelShown(t *testing.T) {
	cfg := SessionConfig{
		Hooks: HookUIConfig{Show: true, ShowOutput: true},
	}
	m := NewModel(nil, "banner", nil, cfg)
	m.recordHook(hookReportMsg{
		report: settings.HookReport{
			Event:    nacelle.AfterToolCall,
			Tool:     "edit_file",
			Command:  "graft blast --no-refresh",
			Label:    "post-edit hook",
			Duration: 15 * time.Millisecond,
			Stdout:   "blast radius: 2 files",
		},
	})

	output := strings.Join(spoken(m), "\n")
	if !strings.Contains(output, "post-edit hook") {
		t.Errorf("got %q, want it to show the hook label", output)
	}
	if !strings.Contains(output, "blast radius: 2 files") {
		t.Errorf("got %q, want it to contain preview output", output)
	}
}

func TestHookExecutionDeniedAndFailureShown(t *testing.T) {
	cfg := SessionConfig{
		Hooks: HookUIConfig{Show: true, ShowOutput: true},
	}
	m := NewModel(nil, "banner", nil, cfg)
	m.recordHook(hookReportMsg{
		report: settings.HookReport{
			Event:    nacelle.BeforeToolCall,
			Tool:     "run_command",
			Command:  "check_policy.sh",
			Duration: 5 * time.Millisecond,
			Denied:   true,
			Reason:   "policy violation",
		},
	})
	m.recordHook(hookReportMsg{
		report: settings.HookReport{
			Event:    nacelle.SessionStart,
			Command:  "init.sh",
			Duration: 8 * time.Millisecond,
			ExitCode: 1,
			Err:      errors.New("exit status 1"),
			Stderr:   "init script crashed",
		},
	})

	output := strings.Join(spoken(m), "\n")
	if !strings.Contains(output, "denied: policy violation") {
		t.Errorf("got %q, want denial reason", output)
	}
	if !strings.Contains(output, "hook failed: exit status 1") {
		t.Errorf("got %q, want failure error", output)
	}
	if !strings.Contains(output, "init script crashed") {
		t.Errorf("got %q, want stderr output preview", output)
	}
}
