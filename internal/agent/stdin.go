package agent

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// stdinPrompt reads the first line of stdin when the terminal is not
// interactive, for piped usage: `echo "list files" | bulle`.
func stdinPrompt() (string, error) {
	data, err := readStdinFirstLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(data), nil
}

// readStdinFirstLine reads up to the first newline from stdin.
func readStdinFirstLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
