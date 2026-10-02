package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

func applyJob(config settings.Config, job settings.CronJob) settings.Config {
	cfg := config
	if job.Workdir != "" {
		cfg.Root = expandHome(job.Workdir)
	}
	cfg = mergeJobConfig(cfg, job)
	cfg.Gates = append(cfg.Gates, job.Gates...)
	bash := false
	if job.Commands != nil {
		bash = *job.Commands
	}
	if job.Toggles.Bash != nil {
		bash = *job.Toggles.Bash
	}
	cfg.Bash = &bash
	approve := false
	cfg.ApproveTools = &approve
	return cfg
}

func jobHooks(job settings.CronJob) (map[nacelle.HookPoint][]nacelle.Hook, error) {
	if len(job.Hooks) == 0 {
		return nil, nil
	}
	return settings.BuildHooks(job.Hooks)
}

func jobContext(timeout string) (context.Context, context.CancelFunc) {
	if timeout == "" {
		return context.Background(), func() {}
	}
	d, err := time.ParseDuration(timeout)
	if err != nil || d <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), d)
}

func ensureTrusted(f settings.JobFile) error {
	ok, err := settings.IsTrusted(f.Path, f.Raw)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return fmt.Errorf("job %q is not trusted: review %s, then run `bulle cron trust %s`", f.Job.Name, f.Path, f.Job.Name)
}

func loadExecutableJob(name string) (settings.CronJob, settings.Config, error) {
	config, files, err := loadCronState()
	if err != nil {
		return settings.CronJob{}, settings.Config{}, err
	}
	f, err := findJobFile(files, name)
	if err != nil {
		return settings.CronJob{}, settings.Config{}, err
	}
	if err := ensureTrusted(f); err != nil {
		return settings.CronJob{}, settings.Config{}, err
	}
	if f.Job.Prompt == "" {
		return settings.CronJob{}, settings.Config{}, fmt.Errorf("job %q has no prompt: nothing to run", name)
	}
	if err := validateDelivery(f.Job.Delivery); err != nil {
		return settings.CronJob{}, settings.Config{}, err
	}
	cfg, err := jobConfig(config, f.Job)
	if err != nil {
		return settings.CronJob{}, settings.Config{}, err
	}
	return f.Job, cfg, nil
}

// jobConfig applies a job over the resolved config and re-validates the ladder the
// merge produced. The job's own check ran against the job's ladder filled with
// defaults, not the one the merge actually yields, so a session's own ratios can
// turn a usable job into a ladder that cannot tier — and nothing downstream
// re-reads it.
func jobConfig(config settings.Config, job settings.CronJob) (settings.Config, error) {
	cfg := applyJob(config, job)
	if err := settings.ValidateCompaction(cfg.Compaction, cfg.CompactAt); err != nil {
		return settings.Config{}, err
	}
	return cfg, nil
}

func runCronJob(name string) error {
	ensureUserPath()
	job, cfg, err := loadExecutableJob(name)
	if err != nil {
		return err
	}
	extra, err := jobHooks(job)
	if err != nil {
		return err
	}
	ctx, cancel := jobContext(job.Timeout)
	defer cancel()
	started := time.Now()
	text, stats, runErr := runHeadlessConfigToContext(ctx, os.Stdout, job.Prompt, cfg, extra)
	rec, status := stats.record(job, cfg.Model, started, runErr)
	logText := text
	if runErr != nil && strings.TrimSpace(logText) == "" {
		logText = fmt.Sprintf("Error: %s\n", runErr.Error())
	}
	logErr := appendCronLog(job.Delivery, job.Name, status, logText)
	return errors.Join(runErr, logErr, appendCronRun(rec))
}
