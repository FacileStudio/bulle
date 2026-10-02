package agent

import (
	"fmt"
	"os"
	"strings"
)

func checkVersionFlag(v string) (bool, error) {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" || arg == "-v" {
			fmt.Println("bulle " + v)
			return true, nil
		}
	}
	return false, nil
}

func checkPrintFlag() (bool, error) {
	printArg, handled := extractPrintFlag()
	if !handled {
		return false, nil
	}
	if printArg == "" {
		piped, err := stdinPrompt()
		if err != nil || piped == "" {
			return true, fmt.Errorf("no prompt: neither -print nor stdin provided")
		}
		printArg = piped
	}
	return true, runHeadless(printArg)
}

func extractPrintFlag() (string, bool) {
	value := ""
	filtered := make([]string, 0, len(os.Args))
	filtered = append(filtered, os.Args[0])
	found := false
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-print":
			found = true
			if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "-") {
				value = os.Args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-print="):
			found = true
			value = strings.TrimPrefix(arg, "-print=")
		default:
			filtered = append(filtered, arg)
		}
	}
	os.Args = filtered
	return value, found
}

func stripPrintFlag() string {
	value, _ := extractPrintFlag()
	return value
}
