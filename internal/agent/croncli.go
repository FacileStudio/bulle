package agent

import (
	"encoding/json"
	"fmt"

	"github.com/FacileStudio/bulle/internal/settings"
)

// UsageError marks a malformed cron invocation.
type UsageError struct {
	err error
}

func (e *UsageError) Error() string { return e.err.Error() }

func usagef(format string, args ...any) error {
	return &UsageError{err: fmt.Errorf(format, args...)}
}

func printCronUsage() error {
	fmt.Println(`usage: bulle cron <command> [args]

  list                 show the jobs in ~/.bulle/jobs/
  run <name>           run one job now through the headless path
  trust <name>         review one job file and approve its contents
  install <name>       install one job directly to crontab
  uninstall <name>     remove one job from crontab
  help                 show this text

  --json               with list, print one JSON document instead of the text table`)
	return nil
}

type cronJobJSON struct {
	Name      string `json:"name"`
	When      string `json:"when"`
	Enabled   bool   `json:"enabled"`
	Installed bool   `json:"installed"`
	Commands  bool   `json:"commands"`
	Workdir   string `json:"workdir"`
	Delivery  string `json:"delivery"`
}

func cronJobsJSON(jobs []settings.CronJob) ([]byte, error) {
	crontabContent, _ := readCrontab()
	out := make([]cronJobJSON, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, cronJobJSON{
			Name:      job.Name,
			When:      job.When,
			Enabled:   job.IsEnabled(),
			Installed: isJobInstalled(crontabContent, job.Name),
			Commands:  jobCommands(job),
			Workdir:   job.Workdir,
			Delivery:  job.Delivery,
		})
	}
	return json.Marshal(out)
}

func printCronJSON(jobs []settings.CronJob) error {
	data, err := cronJobsJSON(jobs)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
