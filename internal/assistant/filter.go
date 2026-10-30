package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Runner runs one external command and returns its standard output. Sources
// take it as a value so tests never need the real binaries.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// CmdError is the failure of a command that exited non-zero. Stderr carries
// what the command printed, which is how a missing feature is detected.
type CmdError struct {
	Stderr string
	Err    error
}

func (e *CmdError) Error() string {
	return e.Err.Error() + ": " + strings.TrimSpace(e.Stderr)
}

func (e *CmdError) Unwrap() error { return e.Err }

// ExecRunner is the Runner that goes through os/exec.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, &CmdError{Stderr: stderr.String(), Err: err}
	}
	return out, nil
}

func runJSON(ctx context.Context, run Runner, timeout time.Duration, v any, argv ...string) error {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := run(ctx, argv[0], argv[1:]...)
	if err != nil {
		return fmt.Errorf("%s: %w", argv[0], err)
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("%s: decode: %w", argv[0], err)
	}
	return nil
}

// IsVIP reports whether the address is in the VIP list, ignoring case.
func IsVIP(addr string, vip []string) bool {
	addr = strings.TrimSpace(addr)
	for _, v := range vip {
		if strings.EqualFold(addr, strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}
