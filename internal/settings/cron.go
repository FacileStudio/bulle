package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

// CronJob is one scheduled background run, one file in ~/.bulle/jobs/.
// The external scheduler (systemd/crontab) owns the clock and fires
// `bulle cron run <name>`; bulle only generates the units and interprets
// the schedule for display. Commands and Enabled default to off, the reverse
// of the interactive defaults, because a run no one can approve starts
// shell-less and disarmed.
//
// The embedded groups override the resolved config group by group, member by
// member: a member the job leaves empty falls back to the config value, and
// a job that mentions no group at all runs exactly on the config's settings.
// A group provider.model beats the flat model key when both are set. Hooks
// and Gates append to the config's own.
type CronJob struct {
	Name     string `yaml:"name"`
	When     string `yaml:"when"`
	Prompt   string `yaml:"prompt"`
	Workdir  string `yaml:"workdir"`
	Model    string `yaml:"model"`
	Delivery string `yaml:"delivery"`
	Timeout  string `yaml:"timeout"`
	Commands *bool  `yaml:"commands"`
	Enabled  *bool  `yaml:"enabled"`

	Provider  Provider   `yaml:"provider"`
	Security  Security   `yaml:"security"`
	Toggles   Toggles    `yaml:"tools"`
	Reasoning Reasoning  `yaml:"reasoning"`
	Limits    Limits     `yaml:"limits"`
	Hooks     []HookSpec `yaml:"hooks"`
	Gates     []GateSpec `yaml:"gates"`
}

// JobsDir is the folder one CronJob is read from per .yml file.
func JobsDir() string {
	dir, err := trustDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "jobs")
}

// JobFile is one job file as loaded: the raw bytes the trust store keys on,
// alongside the decoded job.
type JobFile struct {
	Path string
	Raw  []byte
	Job  CronJob
}

// LoadJobs reads every .yml file in dir as one CronJob, sorted by filename.
// A job whose file stays silent about a name takes the filename stem. A
// missing directory is the ordinary no-jobs case, not an error; two files
// claiming one name are refused, and the error names both files.
func LoadJobs(dir string) ([]CronJob, error) {
	files, err := LoadJobFiles(dir)
	if err != nil {
		return nil, err
	}
	jobs := make([]CronJob, 0, len(files))
	for _, f := range files {
		jobs = append(jobs, f.Job)
	}
	return jobs, nil
}

// LoadJobFiles reads every .yml file in dir as one CronJob with the raw
// bytes kept alongside. The rules are LoadJobs'.
func LoadJobFiles(dir string) ([]JobFile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	seen := make(map[string]string, len(entries))
	files := make([]JobFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		f, err := loadJobFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if first, clash := seen[f.Job.Name]; clash {
			return nil, fmt.Errorf("duplicate job name %q: %s and %s both define it", f.Job.Name, first, f.Path)
		}
		seen[f.Job.Name] = f.Path
		files = append(files, f)
	}
	return files, nil
}

// loadJobFile reads one job file and decodes it strictly, the way the config
// file is decoded: a typo is a hard error naming the file, not a silent
// half-job.
func loadJobFile(path string) (JobFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return JobFile{}, fmt.Errorf("reading %s: %w", path, err)
	}
	job, err := decodeJob(raw, path)
	if err != nil {
		return JobFile{}, err
	}
	return JobFile{Path: path, Raw: raw, Job: job}, nil
}

// decodeJob turns one job file's bytes into a CronJob, defaulting the name
// to the file stem, and validates the job's own compaction block on the way
// through.
//
// A job's limits merge over the already-resolved config, after Settings has had
// its say, so this is the only point that ever sees its ladder — and a job whose
// ratio cannot work would otherwise be the one surface where the mistake is
// still silent.
func decodeJob(raw []byte, path string) (CronJob, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var job CronJob
	if err := decoder.Decode(&job); err != nil && !errors.Is(err, io.EOF) {
		return CronJob{}, &ParseError{Path: path, Err: err}
	}
	if job.Name == "" {
		job.Name = strings.TrimSuffix(filepath.Base(path), ".yml")
	}
	if err := ValidateCompaction(job.Limits.Compaction, job.Limits.CompactAt); err != nil {
		return CronJob{}, err
	}
	return job, nil
}

// IsEnabled returns whether the job is enabled. A nil pointer (the
// default when the job file is silent) means disabled.
func (j CronJob) IsEnabled() bool {
	return j.Enabled != nil && *j.Enabled
}

// repointCronKey rewrites the strict refusal of a top-level cron: key so the
// error says where jobs live now; every other unknown field keeps the
// decoder's own message.
func repointCronKey(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	rewritten := make([]*yaml.UnmarshalError, 0, len(typeErr.Errors))
	for _, item := range typeErr.Errors {
		if strings.Contains(item.Err.Error(), "field cron not found") {
			item = &yaml.UnmarshalError{
				Err:    fmt.Errorf("field cron not found: jobs are one .yml file each under ~/.bulle/jobs/ now"),
				Line:   item.Line,
				Column: item.Column,
			}
		}
		rewritten = append(rewritten, item)
	}
	return &yaml.TypeError{Errors: rewritten}
}
