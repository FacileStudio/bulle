package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/toolview"
)

var chatFreeTools = map[string]bool{
	"read_file": true, "list_directory": true, "find_files": true,
	"search_content": true, "tasks": true, "remember": true,
}

// Asker puts one approval question to the human in a room and returns the
// answer. A non-nil error counts as a denial.
type Asker interface {
	Ask(ctx context.Context, room string, summary string) (bool, error)
}

// ChatApprove builds the approval gate of the chat surface. Read-only tools
// and read-only commands run free, everything else waits for the asker unless
// auto is set, which approves every call that is not a duplicate-key forgery.
func ChatApprove(room string, asker Asker, auto bool) nacelle.Approve {
	return func(ctx context.Context, name string, input json.RawMessage) bool {
		fields, err := toolview.StrictObject(input)
		if errors.Is(err, toolview.ErrDuplicateKey) || foldDuplicate(fields) {
			return false
		}
		if chatFreeTools[name] {
			return true
		}
		summary, free := describe(name, fields, input)
		if free {
			return true
		}
		return settle(ctx, room, asker, auto, summary)
	}
}

func settle(ctx context.Context, room string, asker Asker, auto bool, summary string) bool {
	if auto {
		fmt.Fprintf(os.Stderr, "bulle chat: auto-approved %s\n", clip(summary))
		return true
	}
	if asker == nil {
		return false
	}
	ok, err := asker.Ask(ctx, room, summary)
	return err == nil && ok
}

func foldDuplicate(fields map[string]json.RawMessage) bool {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		for _, other := range keys {
			if strings.EqualFold(key, other) {
				return true
			}
		}
		keys = append(keys, key)
	}
	return false
}

func clip(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len([]rune(text)) > 200 {
		return string([]rune(text)[:200]) + "..."
	}
	return text
}
