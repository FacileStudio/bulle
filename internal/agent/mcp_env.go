package agent

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/nacelle/mcp/client"
)

func ensureUserPath() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	current := os.Getenv("PATH")
	toAdd := missingUserDirs(home, current)
	if len(toAdd) > 0 {
		if err := os.Setenv("PATH", strings.Join(toAdd, string(filepath.ListSeparator))+string(filepath.ListSeparator)+current); err != nil {
			return
		}
	}
	ensureUserEnv()
}

func parseExportLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "export ") {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, "export ")
	key, val, ok := strings.Cut(rest, "=")
	if !ok || key == "" {
		return "", "", false
	}
	if len(val) >= 2 {
		if (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"') {
			val = val[1 : len(val)-1]
		}
	}
	return key, val, true
}

func ensureUserEnv() {
	bin, err := exec.LookPath("tiroir")
	if err != nil {
		return
	}
	out, err := exec.Command(bin, "export").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := parseExportLine(line)
		if !ok || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			continue
		}
	}
}

func missingUserDirs(home, current string) []string {
	candidates := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".local", "share", "mise", "shims"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".local", "share", "bob", "nvim-bin"),
		filepath.Join(home, ".grok", "bin"),
		filepath.Join(home, ".opencode", "bin"),
		"/usr/local/bin",
		"/snap/bin",
	}
	if matches, err := filepath.Glob(filepath.Join(home, ".local", "share", "pi-node", "*", "bin")); err == nil {
		candidates = append(candidates, matches...)
	}
	var missing []string
	for _, c := range candidates {
		if isDir(c) && !strings.Contains(":"+current+":", ":"+c+":") {
			missing = append(missing, c)
		}
	}
	return missing
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func expandServerDef(def client.ServerDef) client.ServerDef {
	def.Command = expandHome(def.Command)
	if def.Dir != "" {
		def.Dir = expandHome(def.Dir)
	}
	if len(def.Args) > 0 {
		args := make([]string, len(def.Args))
		for i, a := range def.Args {
			args[i] = expandHome(a)
		}
		def.Args = args
	}
	if len(def.Env) > 0 {
		expanded := make(map[string]string, len(def.Env))
		for k, v := range def.Env {
			expanded[k] = expandHome(v)
		}
		def.Env = expanded
	}
	return def
}

func expandPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			out = append(out, expandHome(p))
		}
	}
	return out
}

func normalizeCallToolArgs(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("{}")
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		trimmed := strings.TrimSpace(str)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			return json.RawMessage(trimmed)
		}
	}
	return raw
}
