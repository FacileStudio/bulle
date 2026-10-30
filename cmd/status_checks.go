package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/settings"
)

const watchStaleAfter = 35 * time.Minute

func checkConfig() []statusLine {
	lines := []statusLine{}
	if _, err := agent.ChatConfig(); err != nil {
		lines = append(lines, statusLine{statusFail, "config", err.Error()})
	} else {
		lines = append(lines, statusLine{statusOK, "config", settings.ConfigPath()})
	}
	return append(lines, legacyLines()...)
}

func legacyLines() []statusLine {
	var lines []statusLine
	for _, legacy := range []string{".bulle", ".bulle.yml"} {
		path := homeFile(legacy)
		if path == "" {
			continue
		}
		if entries, err := os.ReadDir(path); err == nil && len(entries) > 0 || err != nil && !os.IsNotExist(err) {
			lines = append(lines, statusLine{statusWarn, "legacy", path + " still exists, bulle did not migrate it"})
		}
	}
	return lines
}

func checkChat() []statusLine {
	config, err := agent.ChatConfig()
	if err != nil || config.Chat.Matrix.Enabled == nil || !*config.Chat.Matrix.Enabled {
		return []statusLine{{statusOK, "chat", "disabled"}}
	}
	out, _ := exec.Command("systemctl", "--user", "is-active", chatServiceName).Output()
	if state := strings.TrimSpace(string(out)); state != "active" {
		return []statusLine{{statusFail, "chat", "service " + chatServiceName + " is " + state}}
	}
	sock, err := settings.ChatSocket()
	if err != nil {
		return []statusLine{{statusFail, "chat", err.Error()}}
	}
	conn, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return []statusLine{{statusFail, "chat", "service is active but " + sock + " does not answer"}}
	}
	conn.Close()
	return []statusLine{{statusOK, "chat", "service active, socket " + sock}}
}

func checkAssistant() []statusLine {
	cfg, err := assistantSettings()
	if err != nil {
		return []statusLine{{statusFail, "assistant", err.Error()}}
	}
	lines := cronEntryLines(cfg)
	state, err := settings.StateDir()
	if err != nil {
		return append(lines, statusLine{statusFail, "watch", err.Error()})
	}
	quiet := assistant.InQuiet(time.Now(), cfg, state)
	return append(lines, watchLine(assistant.LastWatch(state), quiet))
}

func watchLine(last time.Time, quiet bool) statusLine {
	if last.IsZero() {
		return statusLine{statusWarn, "watch", "never ran"}
	}
	when := last.Local().Format(time.RFC3339)
	switch {
	case quiet:
		return statusLine{statusOK, "watch", "quiet hours, last run " + when}
	case time.Since(last) > watchStaleAfter:
		return statusLine{statusFail, "watch", "last run " + when + ", cron is not firing"}
	}
	return statusLine{statusOK, "watch", "last run " + when}
}

func cronEntryLines(cfg settings.Assistant) []statusLine {
	entries, err := assistantEntries(cfg, "bulle", "")
	if err != nil {
		return []statusLine{{statusFail, "assistant", err.Error()}}
	}
	crontab, err := agent.SystemCrontab().Read()
	if err != nil {
		return []statusLine{{statusFail, "crontab", err.Error()}}
	}
	var missing []string
	for _, e := range entries {
		if !agent.JobInstalled(crontab, e.Name) {
			missing = append(missing, e.Name)
		}
	}
	if len(missing) > 0 {
		return []statusLine{{statusFail, "crontab", "missing: " + strings.Join(missing, ", ") + " (run: bulle assistant install)"}}
	}
	if legacy := agent.InstalledLegacyJobs(crontab); len(legacy) > 0 {
		return []statusLine{{statusFail, "crontab", "legacy entries duplicate the assistant: " + strings.Join(legacy, ", ")}}
	}
	return []statusLine{{statusOK, "crontab", fmt.Sprintf("%d assistant entries installed", len(entries))}}
}

func checkTriage(c interface{ Context() context.Context }) []statusLine {
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "courrier", "triage", "--all", "--unread", "--limit", "1", "--json").CombinedOutput()
	switch {
	case err == nil:
		return []statusLine{{statusOK, "mail triage", "courrier triage answers"}}
	case strings.Contains(string(out), "does not support triage"):
		return []statusLine{{statusWarn, "mail triage", "server too old, using the local sender-rule fallback"}}
	}
	return []statusLine{{statusFail, "mail triage", strings.TrimSpace(string(out) + " " + err.Error())}}
}
