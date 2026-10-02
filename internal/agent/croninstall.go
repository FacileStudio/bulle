package agent

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/adhocore/gronx"
)

var cronNameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

var cronNameStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green)

func listCronJobs() error {
	config, files, err := loadCronState()
	if err != nil {
		return err
	}
	jobs := make([]settings.CronJob, 0, len(files))
	for _, f := range files {
		jobs = append(jobs, f.Job)
	}
	if settings.DerefBool(config.JSON) {
		return printCronJSON(jobs)
	}
	if len(files) == 0 {
		fmt.Println("no cron jobs in " + settings.JobsDir())
		return nil
	}
	crontabContent, _ := readCrontab()
	for _, f := range files {
		trusted, err := settings.IsTrusted(f.Path, f.Raw)
		if err != nil {
			return err
		}
		job := f.Job
		installed := isJobInstalled(crontabContent, job.Name)
		statusStr := "disabled"
		if job.IsEnabled() {
			statusStr = "enabled"
		}
		fmt.Printf("%s [%s]\n", cronNameStyle.Render(job.Name), statusStr)
		fmt.Printf("when=%s\nenabled=%t\ninstalled=%t\ncommands=%t\ntrusted=%t\nworkdir=%s\ndelivery=%s\n\n",
			job.When, job.IsEnabled(), installed, job.Commands != nil && *job.Commands, trusted, job.Workdir, job.Delivery)
	}
	fmt.Println("(You can update your job files in the ~/.bulle/jobs/ folder)")
	return nil
}

func installCronJob(name string) error {
	return installCronJobOptions(name, false)
}

func resolveInstallJob(name string) (settings.CronJob, string, error) {
	_, files, err := loadCronState()
	if err != nil {
		return settings.CronJob{}, "", err
	}
	f, err := findJobFile(files, name)
	if err != nil {
		return settings.CronJob{}, "", err
	}
	if err := ensureTrusted(f); err != nil {
		return settings.CronJob{}, "", err
	}
	if err := checkCronInstallable(f.Job); err != nil {
		return settings.CronJob{}, "", err
	}
	schedule, err := normalizeCronSchedule(f.Job.When)
	if err != nil {
		return settings.CronJob{}, "", fmt.Errorf("job %q: %w", f.Job.Name, err)
	}
	return f.Job, schedule, nil
}

func installCronJobOptions(name string, printOnly bool) error {
	ensureUserPath()
	job, schedule, err := resolveInstallJob(name)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating bulle: %w", err)
	}
	logsDir := filepath.Join(settings.JobsDir(), "..", "logs")
	logsDir = filepath.Clean(expandHome(logsDir))
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("creating logs directory %s: %w", logsDir, err)
	}
	block := formatCronBlock(job.Name, schedule, bin, logsDir)
	if printOnly {
		fmt.Println(block)
		return nil
	}
	current, err := readCrontab()
	if err != nil {
		return err
	}
	if err := writeCrontab(applyCrontabBlock(current, name, block)); err != nil {
		return err
	}
	fmt.Printf("installed cron job %q (%s) to crontab\n", name, schedule)
	return nil
}

func uninstallCronJob(name string) error {
	ensureUserPath()
	current, err := readCrontab()
	if err != nil {
		return err
	}
	updated, removed := stripCrontabBlock(current, name)
	if !removed {
		fmt.Printf("cron job %q is not installed in crontab\n", name)
		return nil
	}
	if err := writeCrontab(updated); err != nil {
		return err
	}
	fmt.Printf("uninstalled cron job %q from crontab\n", name)
	return nil
}

func normalizeCronSchedule(when string) (string, error) {
	schedule := cmp.Or(when, "daily")
	switch schedule {
	case "daily", "*-*-*", "@daily", "@midnight":
		schedule = "0 0 * * *"
	case "hourly", "@hourly":
		schedule = "0 * * * *"
	case "weekly", "@weekly":
		schedule = "0 0 * * 0"
	case "monthly", "@monthly":
		schedule = "0 0 1 * *"
	case "yearly", "@yearly", "@annually":
		schedule = "0 0 1 1 *"
	}
	if !gronx.IsValid(schedule) {
		return "", fmt.Errorf("invalid cron expression %q", when)
	}
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return "", fmt.Errorf("cron expression %q must have exactly 5 fields, got %d", when, len(fields))
	}
	return schedule, nil
}

func formatCronBlock(name, schedule, bin, logsDir string) string {
	logPath := filepath.Join(logsDir, name+".log")
	return fmt.Sprintf("# BEGIN BULLE JOB %s\n%s %s cron run %s >> %s 2>&1\n# END BULLE JOB %s",
		name, schedule, bin, name, logPath, name)
}

func checkCronInstallable(job settings.CronJob) error {
	if !cronNameRe.MatchString(job.Name) {
		return fmt.Errorf("invalid cron job name %q: cron job names only allow letters, digits, and . _ -", job.Name)
	}
	if job.Enabled == nil || !*job.Enabled {
		return fmt.Errorf("job %q is disabled: run `bulle cron run %s`, confirm the output, then set enabled: true",
			job.Name, job.Name)
	}
	if job.Workdir == "" {
		return fmt.Errorf("job %q has no workdir: without one the run executes in the scheduler's working directory, not the project's; set workdir: /path/to/dir on the job", job.Name)
	}
	return nil
}
