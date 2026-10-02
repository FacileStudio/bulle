package cmd

import (
	"github.com/spf13/cobra"

	"github.com/FacileStudio/bulle/internal/ide"
)

type modelFlags struct {
	backend  string
	model    string
	profile  string
	effort   string
	thinking bool
	budget   int64
}

type sessionFlags struct {
	root           string
	system         string
	additional     string
	cont           bool
	resume         string
	mode           string
	transparent    bool
	noConfig       bool
	printPrompt    string
	showHooks      bool
	showHookOutput bool
	detach         bool
	ide            bool
}

type toolFlags struct {
	bash           bool
	parallelAgents bool
	fetch          bool
	approveTools   bool
	diffs          bool
	tasks          bool
	diagnostics    bool
	searchContent  bool
	findFiles      bool
	mcp            []string
}

type discoveryFlags struct {
	projectContext    bool
	skills            bool
	skillDirs         []string
	trustSkills       bool
	trustHooks        bool
	gatesFile         string
	iterations        int
	compactAt         int64
	json              bool
	maxConcurrency    int
	maxParallelAgents int
}

type cliFlags struct {
	modelFlags
	sessionFlags
	toolFlags
	discoveryFlags
}

func bindFlags(cmd *cobra.Command, f *cliFlags) {
	bindModelFlags(cmd, &f.modelFlags)
	bindSessionFlags(cmd, &f.sessionFlags)
	bindToolFlags(cmd, &f.toolFlags)
	bindDiscoveryFlags(cmd, &f.discoveryFlags)
}

func bindModelFlags(cmd *cobra.Command, f *modelFlags) {
	fl := cmd.Flags()
	fl.StringVar(&f.backend, "backend", "anthropic", "Model provider: anthropic, google, openai, or openrouter")
	fl.StringVar(&f.model, "model", "", "Model identifier, defaulting to provider default")
	fl.StringVar(&f.profile, "profile", "", "Profile name from ~/.bulle/profiles/")
	fl.StringVar(&f.effort, "effort", "", "Reasoning effort: none, minimal, low, medium, high, xhigh, max")
	fl.BoolVar(&f.thinking, "thinking", true, "Stream the model's internal reasoning")
	fl.Int64Var(&f.budget, "reasoning-budget", 0, "Token ceiling for reasoning per turn (0 sets no ceiling)")
}

func bindSessionFlags(cmd *cobra.Command, f *sessionFlags) {
	fl := cmd.Flags()
	fl.StringVar(&f.root, "root", ".", "Directory the file tools may reach")
	fl.StringVar(&f.system, "system-prompt", "", "Override system prompt to guide agent behavior")
	fl.StringVar(&f.additional, "additional-prompt", "", "Extra text appended after the base system prompt")
	fl.BoolVar(&f.cont, "continue", false, "Auto-resume the newest session for this project")
	fl.StringVar(&f.resume, "resume", "", "Resume a specific session by ID or file path")
	fl.StringVar(&f.mode, "mode", "tui", "Interface rendering mode: tui or inline")
	fl.BoolVar(&f.transparent, "transparent-blocks", true, "Drop backdrop on tool result and diff panes")
	fl.BoolVar(&f.noConfig, "no-config", false, "Start with default settings, ignoring ~/.bulle.yml")
	fl.StringVar(&f.printPrompt, "print", "", "Run prompt in headless mode and stream response to stdout")
	fl.BoolVar(&f.showHooks, "show-hooks", true, "Show hook execution in conversation")
	fl.BoolVar(&f.showHookOutput, "show-hook-output", true, "Show hook output preview in conversation")
	fl.BoolVarP(&f.detach, "detach", "d", false, "Launch prompt headlessly in background")
	fl.BoolVar(&f.ide, "ide", false, "Publish session events to an editor over a unix socket")
	ide.BindFlag(&f.ide)
}

func bindToolFlags(cmd *cobra.Command, f *toolFlags) {
	fl := cmd.Flags()
	fl.BoolVar(&f.bash, "bash", true, "Allow the model to execute shell commands")
	fl.BoolVar(&f.parallelAgents, "parallel-agents", true, "Enable parallel delegate tool for concurrent runs")
	fl.BoolVar(&f.fetch, "fetch", true, "Allow the model to fetch and read web pages")
	fl.BoolVar(&f.approveTools, "approve-tools", false, "Prompt for user confirmation before executing tool calls")
	fl.BoolVar(&f.diffs, "diffs", true, "Show unified diffs when the model edits files")
	fl.BoolVar(&f.tasks, "tasks", true, "Enable task planning tool for checklists")
	fl.BoolVar(&f.diagnostics, "diagnostics", true, "Provide compiler and linter diagnostics to the model")
	fl.BoolVar(&f.searchContent, "search-content", true, "Allow the model to search file contents with regex")
	fl.BoolVar(&f.findFiles, "find-files", true, "Allow the model to find files by glob")
	fl.StringSliceVar(&f.mcp, "mcp", nil, "Path to MCP server configuration JSON file (repeatable)")
}

func bindDiscoveryFlags(cmd *cobra.Command, f *discoveryFlags) {
	fl := cmd.Flags()
	fl.BoolVar(&f.projectContext, "project-context", true, "Read CLAUDE.md and AGENTS.md into system prompt")
	fl.BoolVar(&f.skills, "skills", true, "Load skills from ~/.agents/skills and project directories")
	fl.StringSliceVar(&f.skillDirs, "skill-dir", nil, "Additional directory to load skills from (repeatable)")
	fl.BoolVar(&f.trustSkills, "trust-skills", false, "Trust all project .agents/skills directories this run")
	fl.BoolVar(&f.trustHooks, "trust-hooks", false, "Trust current project .bulle/hooks.yml")
	fl.StringVar(&f.gatesFile, "gates-file", "", "YAML file of gate checks the session must pass")
	fl.IntVar(&f.iterations, "max-iterations", 5, "Maximum model turns before asking the user")
	fl.Int64Var(&f.compactAt, "compact-at", 0, "Absolute transcript token threshold for compaction (0 disables; unset derives it from the context window)")
	fl.IntVar(&f.maxConcurrency, "max-concurrency", 16, "Maximum concurrent workers for parallel delegation")
	fl.IntVar(&f.maxParallelAgents, "max-parallel-agents", 16, "Maximum parallel agents running concurrently")
	fl.BoolVar(&f.json, "json", false, "Emit output as JSON document where supported")
}
