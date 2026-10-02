package cmd

import (
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

func collectFlags(cmd *cobra.Command, f *cliFlags) settings.Config {
	var cfg settings.Config
	collectModelFlags(cmd, &f.modelFlags, &cfg)
	collectPromptFlags(cmd, &f.sessionFlags, &cfg)
	collectSessionFlags(cmd, &f.sessionFlags, &cfg)
	collectToolFlags(cmd, &f.toolFlags, &cfg)
	collectDiscoveryFlags(cmd, &f.discoveryFlags, &cfg)
	collectLimitFlags(cmd, &f.discoveryFlags, &cfg)
	return cfg
}

func collectModelFlags(cmd *cobra.Command, f *modelFlags, cfg *settings.Config) {
	fl := cmd.Flags()
	if fl.Changed("backend") {
		cfg.Backend = f.backend
	}
	if fl.Changed("model") {
		cfg.Model = f.model
	}
	if fl.Changed("profile") {
		cfg.Profile = f.profile
	}
	if fl.Changed("effort") {
		cfg.Effort = f.effort
	}
	if fl.Changed("thinking") {
		cfg.Thinking = &f.thinking
	}
	if fl.Changed("reasoning-budget") {
		cfg.Budget = &f.budget
	}
}

// collectPromptFlags covers the pair that must not drift apart: system_prompt
// replaces the base prompt outright, additional_prompt is appended on top of
// whatever is active. Neither is a session or a display choice, so they are
// gathered here rather than padded into collectSessionFlags' own arm count.
func collectPromptFlags(cmd *cobra.Command, f *sessionFlags, cfg *settings.Config) {
	fl := cmd.Flags()
	if fl.Changed("system-prompt") {
		cfg.System = f.system
	}
	if fl.Changed("additional-prompt") {
		cfg.Additional = f.additional
	}
}

func collectSessionFlags(cmd *cobra.Command, f *sessionFlags, cfg *settings.Config) {
	fl := cmd.Flags()
	if fl.Changed("root") {
		cfg.Root = f.root
	}
	if fl.Changed("continue") {
		cfg.Continue = &f.cont
	}
	if fl.Changed("resume") {
		cfg.Resume = &f.resume
	}
	if fl.Changed("mode") {
		cfg.Mode = &f.mode
	}
	if fl.Changed("transparent-blocks") {
		cfg.TransparentBlocks = &f.transparent
	}
	if fl.Changed("no-config") {
		cfg.NoConfig = &f.noConfig
	}
	if fl.Changed("show-hooks") {
		cfg.ShowHooks = &f.showHooks
	}
	if fl.Changed("show-hook-output") {
		cfg.ShowHookOutput = &f.showHookOutput
	}
	if fl.Changed("detach") {
		cfg.Detach = &f.detach
	}
}

func collectToolFlags(cmd *cobra.Command, f *toolFlags, cfg *settings.Config) {
	fl := cmd.Flags()
	if fl.Changed("bash") {
		cfg.Bash = &f.bash
	}
	if fl.Changed("parallel-agents") {
		cfg.ParallelAgents = &f.parallelAgents
	}
	if fl.Changed("fetch") {
		cfg.Fetch = &f.fetch
	}
	if fl.Changed("approve-tools") {
		cfg.ApproveTools = &f.approveTools
	}
	if fl.Changed("diffs") {
		cfg.Diffs = &f.diffs
	}
	if fl.Changed("tasks") {
		cfg.Tasks = &f.tasks
	}
	if fl.Changed("diagnostics") {
		cfg.Diagnostics = &f.diagnostics
	}
	if fl.Changed("search-content") {
		cfg.SearchContent = &f.searchContent
	}
	if fl.Changed("find-files") {
		cfg.FindFiles = &f.findFiles
	}
	if fl.Changed("mcp") {
		cfg.MCPFiles = append(cfg.MCPFiles, f.mcp...)
	}
}

func collectDiscoveryFlags(cmd *cobra.Command, f *discoveryFlags, cfg *settings.Config) {
	fl := cmd.Flags()
	if fl.Changed("project-context") {
		cfg.ProjectContext = &f.projectContext
	}
	if fl.Changed("skills") {
		cfg.Skills = &f.skills
	}
	if fl.Changed("skill-dir") {
		cfg.SkillDirs = f.skillDirs
	}
	if fl.Changed("trust-skills") {
		cfg.TrustSkills = &f.trustSkills
	}
	if fl.Changed("trust-hooks") {
		cfg.TrustHooks = &f.trustHooks
	}
	if fl.Changed("gates-file") {
		cfg.GatesFile = f.gatesFile
	}
}

func collectLimitFlags(cmd *cobra.Command, f *discoveryFlags, cfg *settings.Config) {
	fl := cmd.Flags()
	if fl.Changed("max-iterations") {
		cfg.MaxIterations = &f.iterations
	}
	if fl.Changed("compact-at") {
		cfg.CompactAt = &f.compactAt
	}
	if fl.Changed("max-concurrency") {
		cfg.MaxConcurrency = &f.maxConcurrency
	}
	if fl.Changed("max-parallel-agents") {
		cfg.MaxParallelAgents = &f.maxParallelAgents
	}
	if fl.Changed("json") {
		cfg.JSON = &f.json
	}
}
