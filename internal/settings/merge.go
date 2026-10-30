package settings

import "github.com/FacileStudio/nacelle/mcp/client"

// merge overwrites every setting the layer above actually mentions, and leaves
// the rest alone.
func (c *Config) merge(over Config) {
	c.mergeStrings(over)
	c.mergeToggles(over)
	c.mergeSecurity(over)
	c.mergeUI(over)
	c.mergeLimits(over)
	c.Sandbox.merge(over.Sandbox)
	c.Remote.merge(over.Remote)
	c.Chat.merge(over.Chat)
	c.Assistant.merge(over.Assistant)
	if over.Budget != nil {
		c.Budget = over.Budget
	}
	if len(over.SkillDirs) > 0 {
		c.SkillDirs = over.SkillDirs
	}
	for name, def := range over.MCP {
		if c.MCP == nil {
			c.MCP = map[string]client.ServerDef{}
		}
		c.MCP[name] = def
	}
	c.MCPFiles = append(c.MCPFiles, over.MCPFiles...)
	c.Hooks = append(c.Hooks, over.Hooks...)
	if len(over.Gates) > 0 {
		c.Gates = over.Gates
	}
}

// mergeLimits overwrites every limit over actually mentions.
func (c *Config) mergeLimits(over Config) {
	if over.MaxIterations != nil {
		c.MaxIterations = over.MaxIterations
	}
	if over.CompactAt != nil {
		c.CompactAt = over.CompactAt
	}
	if over.GrindCost != nil {
		c.GrindCost = over.GrindCost
	}
	if over.GrindTokens != nil {
		c.GrindTokens = over.GrindTokens
	}
	if over.GrindContinuations != nil {
		c.GrindContinuations = over.GrindContinuations
	}
	if over.MaxConcurrency != nil {
		c.MaxConcurrency = over.MaxConcurrency
	}
	if over.MaxParallelAgents != nil {
		c.MaxParallelAgents = over.MaxParallelAgents
	}
	c.Compaction.Merge(over.Compaction)
}

// mergeStrings overwrites every string setting over actually mentions. A
// provider's backend, model, base URL and API key merge field by field, so a
// layer that sets only NACELLE_PROVIDER_BASE_URL leaves the rest alone.
func (c *Config) mergeStrings(over Config) {
	if over.Backend != "" {
		c.Backend = over.Backend
	}
	if over.Model != "" {
		c.Model = over.Model
	}
	if over.BaseURL != "" {
		c.BaseURL = over.BaseURL
	}
	if over.APIKey != "" {
		c.APIKey = over.APIKey
	}
	if over.APIKeyCommand != "" {
		c.APIKeyCommand = over.APIKeyCommand
	}
	if over.Effort != "" {
		c.Effort = over.Effort
	}
	if over.Root != "" {
		c.Root = over.Root
	}
	if over.System != "" {
		c.System = over.System
	}
	if over.Additional != "" {
		c.Additional = over.Additional
	}
	if over.Profile != "" {
		c.Profile = over.Profile
	}
}

func mergeBool(dst **bool, src *bool) {
	if src != nil {
		*dst = src
	}
}

// mergeToggles overwrites every *bool setting over actually mentions.
func (c *Config) mergeToggles(over Config) {
	mergeBool(&c.Bash, over.Bash)
	mergeBool(&c.ParallelAgents, over.ParallelAgents)
	mergeBool(&c.Fetch, over.Fetch)
	mergeBool(&c.Thinking, over.Thinking)
	mergeBool(&c.ProjectContext, over.ProjectContext)
	mergeBool(&c.Skills, over.Skills)
	mergeBool(&c.TrustSkills, over.TrustSkills)
	mergeBool(&c.TrustHooks, over.TrustHooks)
	mergeBool(&c.Diffs, over.Diffs)
	mergeBool(&c.Tasks, over.Tasks)
	mergeBool(&c.Diagnostics, over.Diagnostics)
	mergeBool(&c.SearchContent, over.SearchContent)
	mergeBool(&c.FindFiles, over.FindFiles)
}

// mergeSecurity overwrites the security toggles over actually mentions.
func (c *Config) mergeSecurity(over Config) {
	mergeBool(&c.ApproveTools, over.ApproveTools)
	mergeBool(&c.PathIsolation, over.PathIsolation)
	mergeBool(&c.DenyElevation, over.DenyElevation)
	mergeBool(&c.EnvIsolation, over.EnvIsolation)
}

// mergeUI overwrites every pointer field in UI that over actually mentions.
func (c *Config) mergeUI(over Config) {
	mergeBool(&c.GroupTools, over.GroupTools)
	mergeBool(&c.ShowThinking, over.ShowThinking)
	mergeBool(&c.Continue, over.Continue)
	mergeBool(&c.TransparentBlocks, over.TransparentBlocks)
	mergeBool(&c.JSON, over.JSON)
	mergeBool(&c.ShowHooks, over.ShowHooks)
	mergeBool(&c.ShowHookOutput, over.ShowHookOutput)
	mergeBool(&c.HookOutput, over.HookOutput)
	if over.Resume != nil {
		c.Resume = over.Resume
	}
	if over.Mode != nil {
		c.Mode = over.Mode
	}
	if over.PromptPlaceholder != nil {
		c.PromptPlaceholder = over.PromptPlaceholder
	}
	if over.StartMessage != nil {
		c.StartMessage = over.StartMessage
	}
	c.mergeEditor(over)
}

// mergeEditor applies the editor settings from over field by field.
func (c *Config) mergeEditor(over Config) {
	if over.Editor.Editor != "" {
		c.Editor.Editor = over.Editor.Editor
	}
	if over.PromptEditKey != "" {
		c.PromptEditKey = over.PromptEditKey
	}
}
