package agent

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/FacileStudio/bulle/internal/settings"
)

func isJobInstalled(crontab, name string) bool {
	if crontab == "" {
		return false
	}
	beginMarker := "# BEGIN BULLE JOB " + name
	return strings.Contains(crontab, beginMarker) || matchesLegacyCron(crontab, name)
}

func readCrontab() (string, error) {
	cmd := exec.Command("crontab", "-l")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}
	errOut := strings.ToLower(stderr.String() + stdout.String())
	if strings.Contains(errOut, "no crontab") {
		return "", nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("crontab executable not found in PATH: %w", err)
	}
	return "", fmt.Errorf("reading crontab: %w: %s", err, strings.TrimSpace(stderr.String()))
}

func writeCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("crontab executable not found in PATH: %w", err)
		}
		return fmt.Errorf("writing crontab: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func applyCrontabBlock(current, name, newBlock string) string {
	stripped, _ := stripCrontabBlock(current, name)
	trimmed := strings.TrimSpace(stripped)
	if trimmed == "" {
		return newBlock + "\n"
	}
	return trimmed + "\n\n" + newBlock + "\n"
}

func matchesLegacyCron(line, name string) bool {
	_, after, ok := strings.Cut(line, "cron run "+name)
	if !ok {
		return false
	}
	return len(after) == 0 || after[0] == ' ' || after[0] == '\t' || after[0] == '>' || after[0] == ';'
}

func stripCrontabBlock(current, name string) (string, bool) {
	beginMarker := "# BEGIN BULLE JOB " + name
	endMarker := "# END BULLE JOB " + name
	lines := strings.Split(current, "\n")
	var result []string
	inBlock := false
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == beginMarker {
			inBlock = true
			removed = true
			continue
		}
		if inBlock {
			if trimmed == endMarker {
				inBlock = false
			}
			continue
		}
		if matchesLegacyCron(line, name) {
			removed = true
			continue
		}
		result = append(result, line)
	}
	return cleanCrontabLines(result), removed
}

func cleanCrontabLines(lines []string) string {
	var cleaned []string
	lastEmpty := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !lastEmpty && len(cleaned) > 0 {
				cleaned = append(cleaned, "")
				lastEmpty = true
			}
			continue
		}
		cleaned = append(cleaned, line)
		lastEmpty = false
	}
	out := strings.Join(cleaned, "\n")
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	return out + "\n"
}

func findJobFile(files []settings.JobFile, name string) (settings.JobFile, error) {
	for _, f := range files {
		if f.Job.Name == name {
			return f, nil
		}
	}
	return settings.JobFile{}, fmt.Errorf("no cron job named %q", name)
}
