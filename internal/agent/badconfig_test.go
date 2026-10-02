package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

// The coloured report has to survive the exact yaml error shape: one header
// line, then one "line N: field X not found in type Y" per unknown key. Every
// unknown key must reach the screen, since those names are what the person
// fixes or deletes.
func TestBadConfigReportNamesEveryUnknownField(t *testing.T) {
	yamlErr := fmt.Errorf("yaml: unmarshal errors:\n  line 5: field session not found in type settings.Config\n  line 15: field run_command not found in type settings.Toggles")
	bad := &settings.ParseError{Path: "/home/y/.bulle.yml", Err: yamlErr}

	report := badConfigReport(bad)
	for _, want := range []string{"line 5", "session", "line 15", "run_command"} {
		if !strings.Contains(report, want) {
			t.Errorf("report misses %q:\n%s", want, report)
		}
	}
	if !strings.Contains(report, "\033[31m") {
		t.Errorf("report has no colour at all:\n%s", report)
	}
	if strings.Contains(report, "yaml: unmarshal errors") {
		t.Errorf("report echoes the yaml boilerplate:\n%s", report)
	}
}

// A parse error that is not an unknown-field list (bad indentation, tabs) must
// still surface — dimmed, never swallowed.
func TestBadConfigReportKeepsOtherYamlErrors(t *testing.T) {
	bad := &settings.ParseError{Path: "/home/y/.bulle.yml", Err: fmt.Errorf("yaml: line 3: found character that cannot start any token")}
	report := badConfigReport(bad)
	if !strings.Contains(report, "found character") {
		t.Errorf("report dropped a non-field yaml error:\n%s", report)
	}
}
