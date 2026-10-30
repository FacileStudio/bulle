package cmd

import (
	"fmt"
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
	installed := strings.Contains(crontab, "# BEGIN BULLE JOB "+f.Job.Name)
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
