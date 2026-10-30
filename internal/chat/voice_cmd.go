package chat

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func voiceCommand(command string) ([]string, error) {
	if strings.TrimSpace(command) == "" {
		command = voiceDefaultCmd
	}
	argv, err := splitWords(command)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, errors.New("assistant.voice.command is empty")
	}
	expandHome(argv)
	if err := checkModel(argv); err != nil {
		return nil, err
	}
	return argv, nil
}

func expandHome(argv []string) {
	home, _ := os.UserHomeDir()
	for i, a := range argv {
		if a == "~" || strings.HasPrefix(a, "~/") {
			argv[i] = filepath.Join(home, a[1:])
		}
	}
}

func checkModel(argv []string) error {
	for i := 1; i+1 < len(argv); i++ {
		if argv[i] != "-m" {
			continue
		}
		if _, err := os.Stat(argv[i+1]); err != nil {
			return fmt.Errorf("%w (missing model %s)", ErrNoTranscriber, argv[i+1])
		}
	}
	return nil
}

func splitWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	var quote rune
	started := false
	for _, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote, started = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if started || cur.Len() > 0 {
				words = append(words, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, errors.New("assistant.voice.command: unterminated quote")
	}
	if started || cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words, nil
}
