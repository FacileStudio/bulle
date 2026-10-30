package assistant

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const antenneDebounce = 60 * time.Second

// WatchAntenne follows `antenne tail --json` and calls run on each line,
// at most once per 60 seconds. It returns at once when antenne is not
// installed or `antenne status` fails, after logging that once.
func WatchAntenne(ctx context.Context, run func()) {
	if err := exec.CommandContext(ctx, "antenne", "status").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "assistant: antenne trigger off: %v\n", err)
		return
	}
	cmd := exec.CommandContext(ctx, "antenne", "tail", "--json")
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		fmt.Fprintf(os.Stderr, "assistant: antenne trigger off: cannot start tail\n")
		return
	}
	defer cmd.Wait()
	var last time.Time
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if time.Since(last) >= antenneDebounce {
			last = time.Now()
			run()
		}
	}
}
