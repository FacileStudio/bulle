package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FacileStudio/nacelle"
)

type rememberTool struct {
	store *Store
	clock func() time.Time
}

// RememberTool returns the `remember` tool. A nil clock means time.Now. The
// tool result is the exact line saved, so a caller can confirm from it.
func RememberTool(store *Store, clock func() time.Time) nacelle.Tool {
	if clock == nil {
		clock = time.Now
	}
	return rememberTool{store: store, clock: clock}
}

func (rememberTool) Name() string { return "remember" }

func (rememberTool) Description() string {
	return "Save one durable fact about the user or their world to long-term memory " +
		"(preferences, people, decisions, recurring context). Call it when the user " +
		"asks you to remember something or states a lasting fact. Never claim you " +
		"saved something unless this tool returned the saved line."
}

func (rememberTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact": map[string]any{"type": "string", "description": "The fact, one short sentence."},
		},
		"required": []string{"fact"},
	}
}

func (t rememberTool) Run(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Fact string `json:"fact"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("remember: input is not the expected object: %w", err)
	}
	return t.store.Remember(in.Fact, t.clock())
}
