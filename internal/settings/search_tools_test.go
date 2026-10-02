package settings

import (
	"testing"
)

func TestSearchToolsDefaultOn(t *testing.T) {
	written(t, "")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.SearchContent {
		t.Error("search_content default off")
	}
	if !*config.FindFiles {
		t.Error("find_files default off")
	}
}

func TestSearchToolsCanBeDisabledByFile(t *testing.T) {
	written(t, "tools:\n  search_content: false\n  find_files: false\n")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if *config.SearchContent {
		t.Error("search_content: false in file left toggle on")
	}
	if *config.FindFiles {
		t.Error("find_files: false in file left toggle on")
	}
}

func TestSearchToolsEnvironmentBeatsFile(t *testing.T) {
	written(t, "tools:\n  search_content: false\n  find_files: false\n")
	t.Setenv("BULLE_SEARCH_CONTENT", "true")
	t.Setenv("BULLE_FIND_FILES", "true")

	config, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !*config.SearchContent {
		t.Error("BULLE_SEARCH_CONTENT environment did not beat file")
	}
	if !*config.FindFiles {
		t.Error("BULLE_FIND_FILES environment did not beat file")
	}
}
