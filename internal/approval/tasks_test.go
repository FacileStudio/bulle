package approval

import (
	"context"
	"encoding/json"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/bulle/internal/tasks"
)

func TestThePlanToolIsNeverPutInFrontOfAHuman(t *testing.T) {
	gate := New()
	gate.send = func(msg tea.Msg) {
		t.Errorf("the plan tool was put in front of a human: %#v", msg)
		if req, ok := msg.(Request); ok {
			req.Decision <- Denied
		}
	}

	input := json.RawMessage(`{"tasks":[{"title":"read the file","status":"in_progress"}]}`)
	if !gate.Ask(context.Background(), tasks.NewTool().Name(), input) {
		t.Fatal("the plan tool was refused")
	}
}

func TestThePlanToolIsStillRefusedWhenItsInputIsAmbiguous(t *testing.T) {
	gate := New()
	gate.send = func(tea.Msg) {
		t.Fatal("an unrenderable call was put in front of a human anyway")
	}

	input := json.RawMessage(`{"tasks":[],"tasks":[{"title":"x","status":"pending"}]}`)
	if gate.Ask(context.Background(), tasks.NewTool().Name(), input) {
		t.Error("a plan whose input has two values for one key was approved")
	}
}
