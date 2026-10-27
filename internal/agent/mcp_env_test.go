package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle/mcp/client"
)

func TestExpandPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{"~/.claude.json", "/var/mcp.json", ""}
	got := expandPaths(paths)
	wantFirst := filepath.Join(home, ".claude.json")
	if len(got) != 2 || got[0] != wantFirst || got[1] != "/var/mcp.json" {
		t.Errorf("expandPaths = %v, want [%s, /var/mcp.json]", got, wantFirst)
	}
}

func TestExpandServerDef(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	def := client.ServerDef{
		Command: "~/bin/server",
		Dir:     "~/workspace",
		Args:    []string{"run", "~/script.js"},
		Env:     map[string]string{"FILE": "~/data.json", "NAME": "val"},
	}

	expanded := expandServerDef(def)
	if expanded.Command != filepath.Join(home, "bin", "server") {
		t.Errorf("command = %q", expanded.Command)
	}
	if expanded.Dir != filepath.Join(home, "workspace") {
		t.Errorf("dir = %q", expanded.Dir)
	}
	if len(expanded.Args) != 2 || expanded.Args[1] != filepath.Join(home, "script.js") {
		t.Errorf("args = %v", expanded.Args)
	}
	if expanded.Env["FILE"] != filepath.Join(home, "data.json") || expanded.Env["NAME"] != "val" {
		t.Errorf("env = %v", expanded.Env)
	}
}

func TestEnsureUserPath(t *testing.T) {
	ensureUserPath()
	current := os.Getenv("PATH")
	home, _ := os.UserHomeDir()
	localBin := filepath.Join(home, ".local", "bin")
	if info, err := os.Stat(localBin); err == nil && info.IsDir() {
		if !strings.Contains(":"+current+":", ":"+localBin+":") {
			t.Errorf("PATH = %s, want %s included", current, localBin)
		}
	}
}

func TestParseExportLine(t *testing.T) {
	key, val, ok := parseExportLine("export FOO='bar'")
	if !ok || key != "FOO" || val != "bar" {
		t.Fatalf("parse single quote export failed: key=%q val=%q ok=%v", key, val, ok)
	}
	key, val, ok = parseExportLine("export BAZ=\"qux\"")
	if !ok || key != "BAZ" || val != "qux" {
		t.Fatalf("parse double quote export failed: key=%q val=%q ok=%v", key, val, ok)
	}
	key, val, ok = parseExportLine("export RAW=value")
	if !ok || key != "RAW" || val != "value" {
		t.Fatalf("parse raw export failed: key=%q val=%q ok=%v", key, val, ok)
	}
	_, _, ok = parseExportLine("invalid line")
	if ok {
		t.Fatal("expected ok=false for non-export line")
	}
}
