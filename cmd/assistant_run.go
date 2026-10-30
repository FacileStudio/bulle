package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/assistant"
)

func runAssistantWatch(ctx context.Context) error {
	deps, err := assistantDeps()
	if err != nil {
		return err
	}
	started := time.Now()
	res, err := assistant.Watch(ctx, deps)
	return recordAssistantRun(agent.AssistantRun{
		Name: "assistant-watch", Started: started, Surfaced: len(res.Surfaced),
		ModelCalled: res.ModelCalled, Delivery: res.Delivery,
	}, err)
}

func runAssistantDigest(ctx context.Context, name string) error {
	deps, err := assistantDeps()
	if err != nil {
		return err
	}
	started := time.Now()
	res, err := assistant.Digest(ctx, deps, name)
	return recordAssistantRun(agent.AssistantRun{
		Name: "assistant-digest-" + name, Started: started, Surfaced: len(res.Surfaced),
		ModelCalled: res.ModelCalled, Delivery: res.Delivery,
	}, err)
}

func recordAssistantRun(rec agent.AssistantRun, runErr error) error {
	rec.Status = "ok"
	if runErr != nil {
		rec.Status, rec.Error = "failed", runErr.Error()
	} else if rec.Delivery == assistant.DeliveryFailed {
		rec.Status = "failed"
		runErr = errors.New("assistant: delivery failed")
	}
	if err := agent.AppendAssistantRun(rec); err != nil {
		fmt.Fprintf(os.Stderr, "assistant: recording run: %v\n", err)
	}
	return runErr
}
