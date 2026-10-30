package agent

import "strings"

func hasMarker(crontab, name string) bool {
	marker := "# BEGIN BULLE JOB " + name
	for line := range strings.SplitSeq(crontab, "\n") {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

// JobInstalled reports whether the crontab holds the BULLE block of name.
func JobInstalled(crontab, name string) bool {
	return hasMarker(crontab, name)
}
