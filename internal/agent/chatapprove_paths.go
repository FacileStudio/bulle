package agent

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/FacileStudio/bulle/internal/settings"
)

var (
	flagArg     = regexp.MustCompile(`^-[A-Za-z]+$`)
	plainPath   = regexp.MustCompile(`^[A-Za-z0-9_.~/-]+$`)
	plainText   = regexp.MustCompile(`^[A-Za-z0-9 .,:_/@%+=-]+$`)
	doubleBad   = "\"$`\\\n\r"
	journalRoot = filepath.Join("Documents", "NuagePersonal", "Wiki", "Journal")
)

func memoryRoot() string {
	dir, err := settings.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "memory")
}

func readPaths(home string, args []string) bool {
	paths := 0
	for _, a := range args {
		if flagArg.MatchString(a) {
			continue
		}
		if !under(home, a, memoryRoot(), journalRoot) {
			return false
		}
		paths++
	}
	return paths > 0
}

func expand(home, p string) string {
	if p == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

func under(home, p string, roots ...string) bool {
	if !filepath.IsAbs(expand(home, p)) {
		return false
	}
	full := filepath.Clean(expand(home, p))
	for _, root := range roots {
		if root == "" {
			continue
		}
		dir := root
		if !filepath.IsAbs(root) {
			dir = filepath.Join(home, root)
		}
		if full == dir || strings.HasPrefix(full, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func journalAppend(home, command string) bool {
	rest, ok := strings.CutPrefix(command, "echo ")
	if !ok {
		return false
	}
	i := strings.LastIndex(rest, " >> ")
	if i < 0 {
		return false
	}
	text, path := strings.TrimSpace(rest[:i]), strings.TrimSpace(rest[i+4:])
	if !plainPath.MatchString(path) || !safeText(text) {
		return false
	}
	full := filepath.Clean(expand(home, path))
	dir := filepath.Join(home, journalRoot)
	return filepath.IsAbs(full) && strings.HasPrefix(full, dir+string(filepath.Separator))
}

func safeText(t string) bool {
	if len(t) >= 2 && t[0] == '\'' && t[len(t)-1] == '\'' {
		return !strings.ContainsAny(t[1:len(t)-1], "'\n\r")
	}
	if len(t) >= 2 && t[0] == '"' && t[len(t)-1] == '"' {
		return !strings.ContainsAny(t[1:len(t)-1], doubleBad)
	}
	return plainText.MatchString(t)
}
