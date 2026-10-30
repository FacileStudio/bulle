package agent

import (
	"os"
	"regexp"
	"strings"
)

const unsafeChars = "\n\r;&|`$()<>\\*?[]{}!#"

var (
	workspaceID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	dateArg     = regexp.MustCompile(`^(-u|-I|--iso-8601|\+[A-Za-z0-9%:_/.-]+)$`)
)

// ReadOnly reports whether a run_command string is on the exact read-only
// allowlist, or is the single permitted Journal append form. Any ambiguity
// returns false, which means the command needs an approval.
func ReadOnly(command string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	return readOnlyIn(home, command)
}

func readOnlyIn(home, command string) bool {
	command = strings.TrimSpace(command)
	if journalAppend(home, command) {
		return true
	}
	if strings.ContainsAny(command, unsafeChars) {
		return false
	}
	args, ok := words(command)
	if !ok || len(args) == 0 {
		return false
	}
	for _, a := range args {
		if strings.Index(a, "~") > 0 {
			return false
		}
	}
	return allowedWords(home, args)
}

func words(s string) ([]string, bool) {
	var out []string
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
		case r == ' ' || r == '\t':
			if started || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, false
	}
	if started || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, true
}

func allowedWords(home string, a []string) bool {
	switch a[0] {
	case "date":
		return len(a) == 1 || (len(a) == 2 && dateArg.MatchString(a[1]))
	case "cat", "ls":
		return readPaths(home, a[1:])
	case "courrier":
		return courrierOK(a)
	case "agenda":
		return agendaOK(a)
	case "opus":
		return opusOK(a)
	case "mycelium":
		return myceliumOK(a)
	}
	return false
}
