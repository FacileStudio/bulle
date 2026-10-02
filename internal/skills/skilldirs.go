package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// extraSkills reads every skill under each of dirs — another tool's own
// skills folder, most often, named so its skills reach the model without
// moving or copying anything. No trust decision applies, for the same
// reason globalSkills has none: naming a directory here is something only
// the person running bulle, on their own machine, can do in the first
// place.
func extraSkills(dirs []string) ([]skill, []string) {
	var found []skill
	var problems []string
	for _, dir := range dirs {
		s, p := skillsIn(ExpandHome(dir))
		found = append(found, s...)
		problems = append(problems, p...)
	}
	return found, problems
}

// globalSkills reads every skill under ~/.agents/skills/ — the same
// cross-vendor path skills.go's sibling in context.go reads ~/.agents/
// AGENTS.md from. No trust decision applies: this is the user's own
// machine, and nothing here crossed a boundary the user did not control.
func globalSkills() ([]skill, []string) {
	dir := globalSkillsDir()
	if dir == "" {
		return nil, nil
	}
	return skillsIn(dir)
}

// globalSkillsDir is ~/.agents/skills, or "" on a machine with no resolvable
// home directory. It exists so projectSkillContainers can recognise the one
// path it must not offer as a project container.
func globalSkillsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agents", "skills")
}

// loadTrustedContainers loads the skills of every container the trust store
// accepts and names the ones it does not, so the person running bulle can
// review what they are being asked to trust.
func loadTrustedContainers(containers []string, store map[string]trustRecord) (skipped []string, found []skill, problems []string) {
	for _, dir := range containers {
		if !trusted(store, dir) {
			skipped = append(skipped, dir)
			continue
		}
		in, dirProblems := skillsIn(dir)
		found = append(found, in...)
		problems = append(problems, dirProblems...)
	}
	return skipped, found, problems
}

// ExpandHome resolves a leading "~" the way a shell would.
func ExpandHome(dir string) string {
	return expandHome(dir)
}

// expandHome resolves a leading "~" the way a shell would. A flag's own
// argument never needs this — the shell already expanded it before bulle
// saw it — but ~/.bulle.yml and an environment variable set by anything
// that isn't a shell (a service manager's Environment=, for one) go through
// no shell at all, so the same "~/.claude/skills" would silently work from
// one source and not another without this.
func expandHome(dir string) string {
	if dir != "~" && !strings.HasPrefix(dir, "~/") {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	return filepath.Join(home, strings.TrimPrefix(dir, "~"))
}
