package agent

import (
	"path/filepath"
	"testing"
)

func TestExtractDeliverable(t *testing.T) {
	cases := map[string]string{
		"":                              "",
		"all reasoning, OUTPUT NOTHING": "",
		"think <deliver> hi </deliver>": "hi",
		"<deliver>a</deliver> x <deliver>b</deliver>": "b",
		"<deliver>unclosed":                           "",
	}
	for in, want := range cases {
		if got := extractDeliverable(in); got != want {
			t.Errorf("extractDeliverable(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAppendCronLogMatrixWithoutTagIsSilent(t *testing.T) {
	t.Setenv("BULLE_CHAT_SOCK", filepath.Join(t.TempDir(), "nonexistent.sock"))
	if err := appendCronLog("matrix", "daily", cronOK, "Wait, let me reconsider..."); err != nil {
		t.Fatalf("untagged output must be dropped, got %v", err)
	}
}
