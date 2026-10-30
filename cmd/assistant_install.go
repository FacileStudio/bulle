package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/settings"
)

func assistantEntries(a settings.Assistant, bin, logsDir string) ([]agent.CronEntry, error) {
	a = a.WithDefaults()
	line := func(name, args string) string {
		return fmt.Sprintf("%s %s >> %s 2>&1", cronQuote(bin), args, cronQuote(filepath.Join(logsDir, name+".log")))
	}
	entries := []agent.CronEntry{
		{Name: "assistant-tz", Command: "CRON_TZ=" + cronQuote(a.Timezone)},
		{Name: "assistant-watch", Schedule: "*/15 * * * *", Command: line("assistant-watch", "assistant watch")},
	}
	for _, name := range assistant.DigestNames() {
		t, err := time.Parse("15:04", a.Digests[name])
		if err != nil {
			return nil, fmt.Errorf("digest %q: invalid time %q, want HH:MM", name, a.Digests[name])
		}
		cron := fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour())
		id := "assistant-digest-" + name
		entries = append(entries, agent.CronEntry{Name: id, Schedule: cron, Command: line(id, "assistant digest "+name)})
	}
	return append(entries, agent.CronEntry{
		Name: "assistant-consolidate", Schedule: "30 3 * * *",
		Command: line("assistant-consolidate", "assistant consolidate"),
	}), nil
}

func runAssistantInstall(replaceLegacy bool) error {
	cfg, err := assistantSettings()
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating bulle: %w", err)
	}
	logs := settings.LogDir()
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return fmt.Errorf("creating logs directory %s: %w", logs, err)
	}
	return installAssistantEntries(agent.SystemCrontab(), cfg, bin, logs, replaceLegacy)
}

func installAssistantEntries(ct agent.Crontab, cfg settings.Assistant, bin, logs string, replaceLegacy bool) error {
	entries, err := assistantEntries(cfg, bin, logs)
	if err != nil {
		return err
	}
	if err := agent.InstallCronEntries(ct, entries, replaceLegacy); err != nil {
		return err
	}
	fmt.Printf("installed %d assistant cron entries\n", len(entries))
	return nil
}
