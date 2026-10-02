package settings

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// keyCommandTimeout bounds one api_key_command. A key command that stops to ask
// a question, wait on a lock or reach an unreachable network would otherwise
// hang the run before its first turn, where the failure reads as bulle being
// slow rather than as a command nobody answered.
const keyCommandTimeout = 10 * time.Second

// ResolveKeys fills each api key a command is responsible for.
//
// An api_key_command exists so that a config file can carry no secret: the file
// names how to obtain the key and the process that owns it decides whether to
// hand one over. That makes it a source rather than an override — a key any
// layer already supplied wins, whether that layer is a flag, the environment, a
// file or a profile, and a command runs only to fill what is still empty. So a
// TYPESAFE_API_KEY already exported on a machine keeps working untouched and the
// command is never run to disagree with it, and the command runs only on the
// paths that actually speak to a provider: a read-only invocation like
// `bulle list` never runs it, so a locked secret store cannot break inspection.
//
// A command that fails, times out or prints nothing is a load error naming the
// field, never an empty key. An empty key reaches the backend as "no credential"
// and reads exactly like a config that simply forgot one, which is the failure
// this whole setting exists to avoid.
func ResolveKeys(c *Config) error {
	if c.APIKey == "" {
		key, err := KeyFromCommand(c.APIKeyCommand)
		if err != nil {
			return &ParseError{Path: "provider.api_key_command", Err: err}
		}
		c.APIKey = key
	}
	judge := &c.Compaction.Judge
	if judge.APIKey == "" {
		key, err := KeyFromCommand(judge.APIKeyCommand)
		if err != nil {
			return &ParseError{Path: "limits.compaction.judge.api_key_command", Err: err}
		}
		judge.APIKey = key
	}
	return nil
}

// KeyFromCommand runs one key command through the shell and returns the key it
// printed, or an empty string when there is no command to run.
//
// The command goes through `sh -c`, the same way a hook's does, because the
// point of the setting is to name a program with its arguments; the config file
// holding it is already trusted to spawn MCP servers, gates and hooks. Its stdin
// is left closed rather than inherited, so a command that prompts for a
// passphrase fails against end-of-file instead of seizing the terminal or
// waiting forever behind a timeout.
//
// Only stdout becomes the key, and only its first line: trimming is forgiving
// about the trailing newline every `get` prints. stderr is read for the error
// message and never for the key, so a command that warns on stderr still works
// while a key is never taken from a stream a warning could share.
func KeyFromCommand(command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", expandTilde(command))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("%s timed out after %s", command, keyCommandTimeout)
	}
	if err != nil {
		return "", fmt.Errorf("%s failed: %w%s", command, err, stderrNote(stderr.String()))
	}
	key := strings.TrimSpace(stdout.String())
	if key == "" {
		return "", fmt.Errorf("%s printed no key", command)
	}
	return key, nil
}

// stderrNote folds a failed command's stderr into an error suffix, keeping the
// first line only. A key command that fails usually says why in one line, and
// the rest is scrollback nobody reads in an error message.
func stderrNote(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if line == "" {
		return ""
	}
	return ": " + line
}
