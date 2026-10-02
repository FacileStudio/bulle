package agent

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/bulle/internal/tui"
)

const configDocsURL = "https://github.com/FacileStudio/bulle/blob/main/docs/configuration.md"

var ansiField = regexp.MustCompile(`line (\d+): field (\S+) not found in type (\S+)`)

// badConfigReport renders a settings parse failure for a terminal: the path in
// red, each unknown-field line with its line number dim and field name yellow.
func badConfigReport(bad *settings.ParseError) string {
	var out strings.Builder
	out.WriteString("\033[31m")
	out.WriteString(bad.Path)
	out.WriteString(" is not a valid configuration\033[0m\n")
	for line := range strings.SplitSeq(bad.Err.Error(), "\n") {
		if m := ansiField.FindStringSubmatch(line); m != nil {
			fmt.Fprintf(&out, "  \033[2mline %s:\033[0m field \033[33m%s\033[0m is not a setting (in %s)\n", m[1], m[2], m[3])
			continue
		}
		if line != "" && line != "yaml: unmarshal errors:" {
			fmt.Fprintf(&out, "  \033[2m%s\033[0m\n", line)
		}
	}
	return out.String()
}

// confirmDefaultSettings asks the person whether to boot without the file. It
// reads one line from stdin; anything but yes (case-insensitive, first letter)
// counts as no, the safe default.
func confirmDefaultSettings() bool {
	fmt.Fprint(os.Stderr, "\033[1mstart with default settings? [y/N] \033[0m")
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// refusedConfig is the message printed when the person declines the default
// boot: where the docs are, and the flag that skips the file entirely.
func refusedConfig(bad *settings.ParseError) error {
	return &UsageError{err: fmt.Errorf("fix %s, or start with -no-config; every setting is documented at %s", bad.Path, configDocsURL)}
}

// bootOrAsk builds the UI session. An invalid settings file gets a coloured
// report and one yes/no prompt: yes boots with defaults, no exits with the
// documentation link and the -no-config escape hatch.
func bootOrAsk(v string) (*tui.UISession, func(), error) {
	sess, cleanup, err := buildUISession(v, false)
	if err == nil {
		return sess, cleanup, nil
	}
	var bad *settings.ParseError
	if !errors.As(err, &bad) {
		return nil, nil, err
	}
	fmt.Fprintln(os.Stderr, badConfigReport(bad))
	if !confirmDefaultSettings() {
		return nil, nil, refusedConfig(bad)
	}
	return buildUISession(v, true)
}
