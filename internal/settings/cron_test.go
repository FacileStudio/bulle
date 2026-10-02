package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJob drops one job file into a jobs folder.
func writeJob(t *testing.T, dir, name, yaml string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadJobsReadsOneJobPerFile checks the folder contract: one file is one
// job, the stem names it when the file stays silent, a name: key overrides,
// and an override group decodes into its own struct.
func TestLoadJobsReadsOneJobPerFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := JobsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJob(t, dir, "daily.yml", "when: \"*/15 * * * *\"\nprompt: dawn summary\nprovider:\n  model: haiku\n")
	writeJob(t, dir, "weekly.yml", "name: weekly-digest\nprompt: digest\nenabled: true\n")

	jobs, err := LoadJobs(dir)
	if err != nil {
		t.Fatalf("LoadJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("loaded %d jobs, want 2: %+v", len(jobs), jobs)
	}
	if jobs[0].Name != "daily" {
		t.Errorf("stem naming: jobs[0].Name = %q, want daily", jobs[0].Name)
	}
	if jobs[0].When != "*/15 * * * *" || jobs[0].Provider.Model != "haiku" {
		t.Errorf("daily job decoded wrong: %+v", jobs[0])
	}
	if jobs[1].Name != "weekly-digest" {
		t.Errorf("name key: jobs[1].Name = %q, want weekly-digest", jobs[1].Name)
	}
	if jobs[1].Enabled == nil || !*jobs[1].Enabled {
		t.Errorf("weekly enabled = %+v, want true", jobs[1].Enabled)
	}
}

// TestAMissingJobsFolderLoadsAsNoJobs keeps first-run ordinary: no
// ~/.bulle/jobs/ yet is zero jobs, not an error.
func TestAMissingJobsFolderLoadsAsNoJobs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	jobs, err := LoadJobs(JobsDir())
	if err != nil {
		t.Fatalf("LoadJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("loaded %+v from a folder that does not exist", jobs)
	}
}

// TestDuplicateJobNamesAreRefused checks that two files claiming one name
// stop the whole folder from loading, with an error naming both files.
func TestDuplicateJobNamesAreRefused(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "a.yml", "name: brief\nprompt: one\n")
	writeJob(t, dir, "b.yml", "name: brief\nprompt: two\n")

	_, err := LoadJobs(dir)
	if err == nil {
		t.Fatal("two files claiming one name were accepted")
	}
	for _, want := range []string{"brief", filepath.Join(dir, "a.yml"), filepath.Join(dir, "b.yml")} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err.Error(), want)
		}
	}
}

// TestANameClashingWithAnotherStemIsRefused covers the quieter collision: a
// name: key that equals another file's stem. Both spellings resolve to the
// same job name, so the folder is still ambiguous.
func TestANameClashingWithAnotherStemIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "daily.yml", "name: news\nprompt: one\n")
	writeJob(t, dir, "news.yml", "prompt: two\n")

	if _, err := LoadJobs(dir); err == nil {
		t.Fatal("a name key colliding with another file's stem was accepted")
	}
}

// TestJobFilesDecodeStrictly keeps a typo a hard error naming the file,
// because a silent half-job is a run that fires at 2am missing its settings.
func TestJobFilesDecodeStrictly(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "daily.yml", "frobnicate: true\nprompt: x\n")

	_, err := LoadJobs(dir)
	if err == nil {
		t.Fatal("an unknown key in a job file was accepted")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Path != filepath.Join(dir, "daily.yml") {
		t.Errorf("err = %v; want a ParseError naming the file", err)
	}
}

// TestTopLevelCronKeyIsRefused pins the migration: a config still carrying
// the inline cron: list fails at load with an error pointing at
// ~/.bulle/jobs/, instead of the key being ignored and the scheduler
// quietly stopping.
func TestTopLevelCronKeyIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte("cron:\n  - name: brief\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)

	_, err := Settings("", Config{})
	if err == nil {
		t.Fatal("a config with a top-level cron: key was accepted")
	}
	if !strings.Contains(err.Error(), ".bulle/jobs") {
		t.Errorf("err = %v; want it to point at ~/.bulle/jobs/", err)
	}
}
