package cmd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/memory"
)

func runAssistantConsolidate(ctx context.Context) error {
	deps, err := assistantDeps()
	if err != nil {
		return err
	}
	started := time.Now()
	rec := agent.AssistantRun{Name: "assistant-consolidate", Started: started, ModelCalled: true}
	store, err := memory.Default()
	if err != nil {
		return recordAssistantRun(rec, err)
	}
	report, err := store.Consolidate(ctx, deps.Complete, started, memory.Options{})
	if err != nil {
		return recordAssistantRun(rec, err)
	}
	rec.Surfaced = len(report.Added)
	if len(report.Added) > 0 {
		rec.Delivery = postRetained(ctx, report)
	}
	return recordAssistantRun(rec, nil)
}

func postRetained(ctx context.Context, report memory.Report) string {
	quiet := false
	resp, err := chat.SendIPC(ctx, chat.IPCRequest{
		Action: "send", Text: "Retenu cette nuit : " + strings.Join(report.Added, " ; "),
		Notify: &quiet, Record: true,
	})
	if err == nil && !resp.OK {
		err = errors.New(resp.Error)
	}
	if err != nil {
		return "failed"
	}
	return "ok"
}
