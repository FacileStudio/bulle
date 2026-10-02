package agent

import "github.com/FacileStudio/bulle/internal/settings"

func mergeJobProvider(cfg *settings.Config, p settings.Provider, model string) {
	if p.Backend != "" {
		cfg.Backend = p.Backend
	}
	if p.BaseURL != "" {
		cfg.BaseURL = p.BaseURL
	}
	if p.APIKey != "" {
		cfg.APIKey = p.APIKey
	}
	if p.APIKeyCommand != "" {
		cfg.APIKeyCommand = p.APIKeyCommand
	}
	if p.Model != "" {
		cfg.Model = p.Model
	} else if model != "" {
		cfg.Model = model
	}
}

func mergeJobSecurity(cfg *settings.Config, s settings.Security) {
	if s.PathIsolation != nil {
		cfg.PathIsolation = s.PathIsolation
	}
	if s.DenyElevation != nil {
		cfg.DenyElevation = s.DenyElevation
	}
	if s.EnvIsolation != nil {
		cfg.EnvIsolation = s.EnvIsolation
	}
}

func mergeJobToggles(cfg *settings.Config, t settings.Toggles) {
	if t.ParallelAgents != nil {
		cfg.ParallelAgents = t.ParallelAgents
	}
	if t.Fetch != nil {
		cfg.Fetch = t.Fetch
	}
	if t.Tasks != nil {
		cfg.Tasks = t.Tasks
	}
	if t.Diagnostics != nil {
		cfg.Diagnostics = t.Diagnostics
	}
	if t.SearchContent != nil {
		cfg.SearchContent = t.SearchContent
	}
	if t.FindFiles != nil {
		cfg.FindFiles = t.FindFiles
	}
}

func mergeJobLimits(cfg *settings.Config, r settings.Reasoning, l settings.Limits) {
	if r.Effort != "" {
		cfg.Effort = r.Effort
	}
	if r.Thinking != nil {
		cfg.Thinking = r.Thinking
	}
	if r.Budget != nil {
		cfg.Budget = r.Budget
	}
	if l.MaxIterations != nil {
		cfg.MaxIterations = l.MaxIterations
	}
	if l.CompactAt != nil {
		cfg.CompactAt = l.CompactAt
	}
	cfg.Compaction.Merge(l.Compaction)
}

func mergeJobConfig(cfg settings.Config, job settings.CronJob) settings.Config {
	mergeJobProvider(&cfg, job.Provider, job.Model)
	mergeJobSecurity(&cfg, job.Security)
	mergeJobToggles(&cfg, job.Toggles)
	mergeJobLimits(&cfg, job.Reasoning, job.Limits)
	return cfg
}
