package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/toolview"
)

const summaryLimit = 300

var chatFreeTools = map[string]bool{
	"read_file": true, "list_directory": true, "find_files": true,
	"search_content": true, "web_fetch": true, "tasks": true, "remember": true,
}

// Asker puts one approval question to the human in a room and returns the
// answer. A non-nil error counts as a denial.
type Asker interface {
	Ask(ctx context.Context, room string, summary string) (bool, error)
}

// ChatApprove builds the approval gate of the chat surface. Read-only tools
// and read-only commands run free, everything else waits for the asker.
func ChatApprove(room string, asker Asker) nacelle.Approve {
	return func(ctx context.Context, name string, input json.RawMessage) bool {
		fields, err := toolview.StrictObject(input)
		if errors.Is(err, toolview.ErrDuplicateKey) {
			return false
		}
		if chatFreeTools[name] {
			return true
		}
		summary, free := describe(name, fields, input)
		if free || asker == nil {
			return free
		}
		ok, err := asker.Ask(ctx, room, summary)
		return err == nil && ok
	}
}

func describe(name string, fields map[string]json.RawMessage, input json.RawMessage) (string, bool) {
	if name == "run_command" {
		var command string
		if json.Unmarshal(fields["command"], &command) == nil && command != "" {
			return "`" + command + "`", ReadOnly(command)
		}
	}
	text := string(input)
	if len(text) > summaryLimit {
		text = text[:summaryLimit] + "..."
	}
	return fmt.Sprintf("l'outil `%s` avec %s", name, text), false
}
