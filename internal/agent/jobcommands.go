package agent

import (
	"github.com/FacileStudio/bulle/internal/settings"
)

// jobCommands reads a job's commands pointer setting with the policy defaults filled in: a job is shell-less until it says otherwise.
func jobCommands(job settings.CronJob) bool {
	return job.Commands != nil && *job.Commands
}
