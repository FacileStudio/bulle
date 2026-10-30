package agent

import (
	"fmt"
	"strings"
)

// LegacyAssistantJobs returns the cron jobs the assistant scheduler replaces.
func LegacyAssistantJobs() []string {
	return []string{"assistant-pulse", "assistant-checkin"}
}

// CronEntry is one crontab block: a name for the BEGIN and END markers, a
// five-field schedule and the full command line.
type CronEntry struct {
	Name     string
	Schedule string
	Command  string
}

// Crontab reads and writes the user crontab. Tests replace both.
type Crontab struct {
	Read  func() (string, error)
	Write func(string) error
}

// SystemCrontab is the Crontab backed by the crontab binary.
func SystemCrontab() Crontab {
	return Crontab{Read: readCrontab, Write: writeCrontab}
}

// FormatEntryBlock renders an entry between its BULLE JOB markers.
// An empty Schedule renders the command alone, which is how an environment
// line such as CRON_TZ=Europe/Paris is managed.
func FormatEntryBlock(e CronEntry) string {
	body := e.Schedule + " " + e.Command
	if e.Schedule == "" {
		body = e.Command
	}
	return fmt.Sprintf("# BEGIN BULLE JOB %s\n%s\n# END BULLE JOB %s", e.Name, body, e.Name)
}

// InstalledLegacyJobs lists the legacy assistant jobs present in the crontab.
func InstalledLegacyJobs(current string) []string {
	var found []string
	for _, name := range LegacyAssistantJobs() {
		if isJobInstalled(current, name) {
			found = append(found, name)
		}
	}
	return found
}

// InstallCronEntries writes every entry into the crontab, replacing blocks of
// the same name. It refuses when a legacy assistant job is installed unless
// replaceLegacy is set, in which case it removes those jobs.
func InstallCronEntries(ct Crontab, entries []CronEntry, replaceLegacy bool) error {
	current, err := ct.Read()
	if err != nil {
		return err
	}
	legacy := InstalledLegacyJobs(current)
	if len(legacy) > 0 && !replaceLegacy {
		return fmt.Errorf("legacy cron jobs installed: %s; rerun with --replace-legacy to remove them", strings.Join(legacy, ", "))
	}
	for _, name := range legacy {
		current, _ = stripCrontabBlock(current, name)
	}
	for _, e := range entries {
		current = applyCrontabBlock(current, e.Name, FormatEntryBlock(e))
	}
	return ct.Write(current)
}
