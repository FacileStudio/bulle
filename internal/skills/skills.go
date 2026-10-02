// Package skills discovers and loads agent capabilities.
package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Skill is one capability catalogued from a SKILL.md — enough for the model
// to decide whether it applies, and where to read the rest.
//
// Only name and description ever reach the system prompt; the rest of a
// skill's directory (scripts, references, assets) is read on demand by the
// model's own read_file call once it decides the skill is worth using. This
// package never reads past the frontmatter itself.
type Skill struct {
	Name, Description, Path string
}

type skill = Skill

// skillFrontmatter is the two fields this package requires, out of every
// field the Agent Skills specification allows (license, compatibility,
// metadata, allowed-tools, disable-model-invocation). Both name and
// description are required by the spec; the rest are validated by neither
// the spec's own reference tooling nor this package in a way that blocks
// loading, so they are not decoded here at all — a field this package never
// reads cannot go stale when the spec adds one.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// SkillsResult is what loading skills produces: text for the system prompt,
// a note for the person watching when a project has skills sitting there
// unloaded because nobody has trusted them yet, and the skills themselves —
// so a caller that wants to run one directly, not just tell the model it
// exists, does not have to load them a second time.
type SkillsResult struct {
	System string
	Notice string
	Skills []Skill

	system string
	notice string
	skills []Skill
}

type skillsResult = SkillsResult

// LoadSkills assembles every skill bulle will tell the model about: every
// skill under ~/.agents/skills/, every skill under a trusted .agents/
// skills/ found walking up from root, and every skill under extraDirs
// (-skill-dir / NACELLE_SKILL_DIRS / skill_dirs — another tool's own skills
// folder, most often). trustNew marks every project-local skills directory
// found on this run as trusted before deciding what loads, which is what
// -trust-skills asks for.
func LoadSkills(root string, trustNew bool, extraDirs []string) SkillsResult {
	return loadSkills(root, trustNew, extraDirs)
}

func loadSkills(root string, trustNew bool, extraDirs []string) skillsResult {
	var found []skill
	var problems []string
	global, globalProblems := globalSkills()
	found = append(found, global...)
	problems = append(problems, globalProblems...)
	extra, extraProblems := extraSkills(extraDirs)
	found = append(found, extra...)
	problems = append(problems, extraProblems...)

	containers := projectSkillContainers(root)
	store, err := loadTrust()
	if err != nil {
		store = map[string]trustRecord{}
	}

	var saveErr error
	if trustNew {
		for _, dir := range containers {
			trust(store, dir)
		}
		if len(containers) > 0 {
			saveErr = saveTrust(store)
		}
	}

	skipped, dirSkills, dirProblems := loadTrustedContainers(containers, store)
	found = append(found, dirSkills...)
	problems = append(problems, dirProblems...)

	sys, not := renderSkills(found), skillNotice(skipped, saveErr, problems)
	return skillsResult{
		System: sys, Notice: not, Skills: found,
		system: sys, notice: not, skills: found,
	}
}

// projectSkillContainers walks from root to the filesystem root, returning
// every .agents/skills directory found along the way — the containers
// requiring trust, not yet the skills inside them.
//
// This walks to the filesystem root rather than stopping at a git repo
// boundary, unlike some other tools' equivalent walk. Consistency with the
// AGENTS.md/CLAUDE.md walk this package already does (context.go) was
// judged more valuable than an exact match to any one other tool, and
// detecting a git root would be one more thing this package has to get
// right for a boundary the trust gate makes low-stakes anyway: a directory
// with nothing to find costs one stat call.
//
// ~/.agents/skills is excluded, and has to be. Walking to the filesystem
// root means passing through $HOME, which is an ancestor of very nearly
// every root anyone runs this from, so once that directory exists the walk
// finds it every time. globalSkills has already loaded it, ungated and by
// design. Offering it here as well would report skills that are loaded as
// "not trusted, nothing loaded", and would load every one of them a second
// time on any run that did trust it.
func projectSkillContainers(root string) []string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}

	global := globalSkillsDir()

	var containers []string
	for dir := abs; ; {
		candidate := filepath.Join(dir, ".agents", "skills")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() && candidate != global {
			containers = append(containers, candidate)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return containers
		}
		dir = parent
	}
}

// skillsIn finds every skill under one directory, recursively: any
// directory holding a SKILL.md is a skill, and its own subtree — scripts,
// references, assets, or a directory that happens to be named like it could
// hold another skill — is not searched further once it has matched.
//
// dir not existing at all is the ordinary case for most of the locations
// this is called with (~/.agents/skills on a machine that has not adopted
// the convention, a project with no .agents/skills of its own) and is
// treated the same as a single unreadable entry inside an otherwise real
// directory: both are nothing to report, not a reason to fail.
//
// Rejected manifests come back as problems — one human-readable line each,
// path first — so a skill that looks installed but never shows up can be
// diagnosed instead of silently swallowed. A manifest that is just a plain
// markdown file with no frontmatter is not a problem.
func skillsIn(dir string) ([]Skill, []string) {
	var found []Skill
	var problems []string
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		manifest := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(manifest); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			return nil
		}
		s, problem := parseSkill(manifest)
		if problem != "" {
			problems = append(problems, manifest+": "+problem)
			return filepath.SkipDir
		}
		if s.Name != "" {
			found = append(found, s)
		}
		return filepath.SkipDir
	}); err != nil {
		return found, problems
	}
	return found, problems
}

// parseSkill reads one SKILL.md's frontmatter. It returns the skill and an
// empty problem when the manifest is loadable, and a non-empty problem
// naming why it was rejected otherwise. A file with no frontmatter at all
// is a plain markdown file, not a broken skill, and gets no problem — the
// distinction a missing name deserves but an ordinary markdown file does
// not.
func parseSkill(path string) (Skill, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Sprintf("unreadable: %v", err)
	}

	block, ok := frontmatter(string(raw))
	if !ok {
		return Skill{}, ""
	}

	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		return Skill{}, fmt.Sprintf("frontmatter does not parse as YAML: %v", err)
	}
	if meta.Name == "" || meta.Description == "" {
		return Skill{}, "frontmatter is missing a required name or description"
	}
	return Skill{Name: meta.Name, Description: singleLine(meta.Description), Path: path}, ""
}

// singleLine collapses every whitespace run to one space. YAML block scalars
// (description: >) carry a trailing newline and folded line breaks into the
// decoded string, and that newline ends up rendered as a blank line in the
// skills menu, so a description is one line by contract.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// frontmatter extracts the YAML block between a file's opening and closing
// --- markers, reporting whether one was found at all.
func frontmatter(content string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	rest := content[len("---\n"):]
	head, _, found := strings.Cut(rest, "\n---")
	if !found {
		return "", false
	}
	return head, true
}
