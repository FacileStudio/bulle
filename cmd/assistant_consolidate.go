package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	report, err := store.Consolidate(ctx, deps.Complete, started, journalOptions(deps.Settings.JournalDir, started))
	if err != nil {
		return recordAssistantRun(rec, err)
	}
	rec.Surfaced = len(report.Added) + len(report.Dropped)
	if len(report.Added)+len(report.Dropped) > 0 {
		rec.Delivery = postRetained(ctx, report)
	}
	return recordAssistantRun(rec, nil)
}

func consolidateText(report memory.Report) string {
	var parts []string
	if len(report.Added) > 0 {
		parts = append(parts, "Retenu cette nuit : "+strings.Join(report.Added, " ; "))
	}
	if len(report.Dropped) > 0 {
		parts = append(parts, "Oublié cette nuit : "+strings.Join(report.Dropped, " ; "))
	}
	return strings.Join(parts, "\n")
}

func postRetained(ctx context.Context, report memory.Report) string {
	quiet := false
	resp, err := chat.SendIPC(ctx, chat.IPCRequest{
		Action: "send", Text: consolidateText(report),
		Notify: &quiet, Record: true,
		Items: []string{"memory:consolidate:" + time.Now().Format("2006-01-02")},
	})
	if err == nil && !resp.OK {
		err = errors.New(resp.Error)
	}
	if err != nil {
		return "failed"
	}
	return "ok"
}

func journalOptions(dir string, now time.Time) memory.Options {
	if dir == "" {
		return memory.Options{}
	}
	dir = expandTilde(dir)
	if _, err := memory.EnsureJournal(dir, now); err != nil {
		fmt.Fprintf(os.Stderr, "assistant: journal template: %v\n", err)
	}
	return memory.Options{JournalPath: memory.JournalPath(dir, now.AddDate(0, 0, -1))}
}
