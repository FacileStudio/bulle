package agent

import "strings"

type flagSet map[string]bool

func positionals(args []string, allowed flagSet) ([]string, bool) {
	var rest []string
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			rest = append(rest, args[i])
			continue
		}
		skip, ok := flagWidth(args[i:], allowed)
		if !ok {
			return nil, false
		}
		i += skip
	}
	return rest, true
}

func flagWidth(args []string, allowed flagSet) (int, bool) {
	name, _, hasValue := strings.Cut(args[0], "=")
	takesValue, known := allowed[name]
	switch {
	case !known || (hasValue && !takesValue):
		return 0, false
	case !takesValue || hasValue:
		return 0, true
	case len(args) < 2 || strings.HasPrefix(args[1], "-"):
		return 0, false
	}
	return 1, true
}
