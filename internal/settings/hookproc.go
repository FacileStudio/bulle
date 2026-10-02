package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"
)

// HookReport records the execution details and outcome of a hook run.
type HookReport struct {
	Event    nacelle.HookPoint
	Tool     string
	Command  string
	Label    string
	Duration time.Duration
	ExitCode int
	Err      error
	Stdout   string
	Stderr   string
	Denied   bool
	Reason   string
}

var hookReports = make(chan HookReport, 128)

// ReportHook records one hook execution report on the global report channel.
func ReportHook(r HookReport) {
	select {
	case hookReports <- r:
	default:
	}
}

// HookReports returns the channel that hook reports are delivered to.
func HookReports() <-chan HookReport {
	return hookReports
}

// execHook builds the hook that runs one spec's command and translates its
// exit back into a decision. A leading ~ in the command is expanded once at
// build time: sh -c does not expand a tilde mid-string, and every config
// file in this ecosystem writes paths that start with one.
func execHook(spec HookSpec) nacelle.Hook {
	command := expandTilde(spec.Run)
	return func(ctx context.Context, ev nacelle.HookEvent) nacelle.HookResult {
		if !spec.Matches(ev.Tool) {
			return nacelle.HookResult{}
		}
		start := time.Now()
		out, errOut, runErr := runCommand(ctx, command, hookPayload{
			Event: string(ev.Point), Tool: ev.Tool,
			Input: ev.Input, Result: ev.Result, Retry: ev.Retry,
		})
		dur := time.Since(start)
		res := interpret(command, ev, runErr, out, errOut)
		ReportHook(HookReport{
			Event:    ev.Point,
			Tool:     ev.Tool,
			Command:  spec.Run,
			Label:    spec.Name,
			Duration: dur,
			ExitCode: exitCode(runErr),
			Err:      runErr,
			Stdout:   string(out),
			Stderr:   string(errOut),
			Denied:   res.Deny != "",
			Reason:   res.Deny,
		})
		return res
	}
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(command string) string {
	if command != "~" && !strings.HasPrefix(command, "~/") {
		return command
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command
	}
	return filepath.Join(home, strings.TrimPrefix(command, "~"))
}

// runCommand executes one spec through sh with the event JSON on stdin.
func runCommand(ctx context.Context, command string, payload hookPayload) (out []byte, errOut []byte, runErr error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bulle hooks: encoding event for %q: %v\n", command, err)
		return nil, nil, err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = bytes.NewReader(encoded)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr = cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), runErr
}

// interpret turns one finished hook process into a decision:
//
//	exit 0        allow; stdout becomes injected context
//	exit 2        deny; stderr is the reason the model reads
//	other failure deny; stderr goes to this process's stderr instead,
//	              because a crash is a bug report for the human, not
//	              instruction-shaped text for the model
//
// Injection lands on AfterToolCall, where there is a result to amend, and on
// SessionStart, where there is no result but the run has not started, so the
// text rides into the conversation on its own. It is skipped on a failed
// tool: prose after an error reads to the model as if the error were handled
// when nothing was.
func interpret(command string, ev nacelle.HookEvent, runErr error, out, errOut []byte) nacelle.HookResult {
	switch {
	case runErr == nil:
		if injects(ev.Point) && len(out) > 0 && ev.Err == nil {
			return nacelle.HookResult{Inject: strings.TrimRight(string(out), "\n")}
		}
		return nacelle.HookResult{}
	case exitCode(runErr) == 2:
		reason := strings.TrimSpace(string(errOut))
		if reason == "" {
			reason = "denied by hook without a reason"
		}
		return nacelle.HookResult{Deny: reason}
	default:
		fmt.Fprintf(os.Stderr, "bulle hooks: %q failed: %v: %s\n",
			command, runErr, strings.TrimSpace(string(errOut)))
		return nacelle.HookResult{Deny: fmt.Sprintf("hook watching %q failed", ev.Tool)}
	}
}

// injects reports the points whose exit-0 stdout reaches the model. The
// other points fire for audit or gating, and their stdout goes unread.
func injects(p nacelle.HookPoint) bool {
	return p == nacelle.AfterToolCall || p == nacelle.SessionStart
}

// exitCode recovers a command's status without importing syscall for it.
func exitCode(err error) int {
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode()
	}
	return -1
}
