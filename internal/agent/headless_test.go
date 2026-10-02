package agent

import (
	"os"
	"strings"
	"testing"
)

func TestStripPrintFlagExtractsPrompt(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		want     string
		wantArgs string
	}{
		{"-print with arg", []string{"bulle", "-print", "hello world"}, "hello world", "bulle"},
		{"-print=equals", []string{"bulle", "-print=hello"}, "hello", "bulle"},
		{"-print alone (stdin)", []string{"bulle", "-print"}, "", "bulle"},
		{"no -print", []string{"bulle", "-model", "abc"}, "", "bulle -model abc"},
		{"-print after other flags", []string{"bulle", "-root", ".", "-print", "hello"}, "hello", "bulle -root ."},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Args
			os.Args = tt.args
			got := stripPrintFlag()
			gotArgs := strings.Join(os.Args, " ")
			os.Args = saved

			if got != tt.want {
				t.Errorf("stripPrintFlag() = %q, want %q", got, tt.want)
			}
			if gotArgs != tt.wantArgs {
				t.Errorf("after strip, os.Args = %q, want %q", gotArgs, tt.wantArgs)
			}
		})
	}
}
