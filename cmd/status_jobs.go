package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/settings"
)

func checkJobs() []statusLine {
	files, err := settings.LoadJobFiles(settings.JobsDir())
	if err != nil {
		return []statusLine{{statusFail, "jobs", err.Error()}}
	}
	crontab, _ := agent.SystemCrontab().Read()
	lines := []statusLine{}
	for _, f := range files {
		lines = append(lines, jobLine(f, crontab))
	}
	if len(lines) == 0 {
		return []statusLine{{statusOK, "jobs", "none defined"}}
	}
	return lines
}

func jobLine(f settings.JobFile, crontab string) statusLine {
	name := "job " + f.Job.Name
	if !f.Job.IsEnabled() {
		return statusLine{statusOK, name, "disabled"}
	}
	trusted, err := settings.IsTrusted(f.Path, f.Raw)
	installed := agent.JobInstalled(crontab, f.Job.Name)
	switch {
	case err != nil:
		return statusLine{statusFail, name, err.Error()}
	case !trusted:
		return statusLine{statusFail, name, "enabled but not trusted (run: bulle cron trust " + f.Job.Name + ")"}
	case !installed:
		return statusLine{statusWarn, name, "enabled and trusted but not in the crontab"}
	}
	return statusLine{statusOK, name, fmt.Sprintf("%s, trusted, installed", f.Job.When)}
}

var logRedirect = regexp.MustCompile(`>>\s*'?([^\s'>]+)`)

func checkCronLogs() []statusLine {
	crontab, err := agent.SystemCrontab().Read()
	if err != nil {
		return nil
	}
	broken := missingLogDirs(crontab)
	if len(broken) == 0 {
		return nil
	}
	return []statusLine{{statusFail, "cron logs", "the shell cannot open the log in " + strings.Join(broken, ", ") +
		", so those entries never run (run: bulle assistant install, then bulle cron install <name>)"}}
}

func missingLogDirs(crontab string) []string {
	var broken []string
	for line := range strings.SplitSeq(crontab, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") || !strings.Contains(line, "bulle") {
			continue
		}
		m := logRedirect.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if _, err := os.Stat(filepath.Dir(m[1])); err != nil {
			broken = append(broken, filepath.Dir(m[1]))
		}
	}
	return broken
}
