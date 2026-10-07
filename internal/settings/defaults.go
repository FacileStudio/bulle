package settings

import (
	"strings"

	"github.com/FacileStudio/bulle/internal/compaction"
)

// DefaultCompactAt is the transcript size, in tokens, a session falls back to
// when it has no compact_at of its own and the backend reports no context
// window to derive one from. An unset compact_at with a known window derives
// its ceiling from the ratio ladder instead.
const DefaultCompactAt int64 = 75_000

// The tier ladder's defaults: soft tombstones history tool results with no model
// call, smart adds one batched judge pass and one ledger summary and forces the
// fold when the gentle one does not land. A backend that reports no context
// window cannot use them — compact_at and DefaultCompactAt are the fallback
// there. They alias the compaction package so the two layers cannot drift.
const (
	DefaultSoftRatio      = compaction.DefaultSoftRatio
	DefaultSmartRatio     = compaction.DefaultSmartRatio
	DefaultPruneThreshold = compaction.DefaultPruneThreshold
)

// The verbatim tail's shipped bounds, aliased the same way the ratios are: a
// floor of one message — the live turn, which no summary may stand in for — and
// a token budget that sizes everything above that floor.
const (
	DefaultKeepTurns  = compaction.DefaultKeepTurns
	DefaultKeepTokens = compaction.DefaultKeepTokens
)

// JudgeProvider names the decision model the compaction judge runs on. jev is
// TypeSafe's System One, the default and the one the shipped thresholds are
// calibrated against; clef is Cloudflare's fine-tune, served through OpenRouter.
// Both are accepted at load; only these two names build a judge.
const (
	JudgeProviderJEV  = "jev"
	JudgeProviderClef = "clef"
)

// Defaults is the bottom layer, and the only one that answers everything.
func Defaults(system string) Config {
	bash, thinking, projectContext, skills, trustSkills, approveTools, trustHooks, diffs, tasks, strict :=
		true, true, true, true, false, false, false, true, true, false
	envIsolation, denyElevation, parallelAgents, diagnostics := false, true, true, true
	iterations, budget, grindCost, grindTokens, grindContinuations, maxConcurrency := 5, int64(0), 0.0, int64(0), 2, 16
	fetch, searchContent, findFiles, groupTools, showThinking, showHooks, showHookOutput := true, true, true, true, true, true, true
	cont, resume, mode, transparent, json := false, "", "tui", true, false
	promptPlaceholder := "Ask something. Esc stops a run, ctrl+c stops or quits, ctrl+\\ forces it."
	startMessage := strings.Join([]string{
		"▄▄          ▄▄ ▄▄       ",
		"██          ██ ██       ",
		"████▄ ██ ██ ██ ██ ▄█▀█▄ ",
		"██ ██ ██ ██ ██ ██ ██▄█▀ ",
		"████▀ ▀██▀█ ██ ██ ▀█▄▄▄ ",
	}, "\n")
	autoSnapshot := false
	chatOff := false
	return Config{
		Provider:  Provider{Backend: "anthropic"},
		Session:   Session{Root: ".", System: system, Continue: &cont, Resume: &resume},
		Toggles:   Toggles{Bash: &bash, ParallelAgents: &parallelAgents, Fetch: &fetch, Tasks: &tasks, Diagnostics: &diagnostics, SearchContent: &searchContent, FindFiles: &findFiles},
		Security:  Security{ApproveTools: &approveTools, PathIsolation: &strict, DenyElevation: &denyElevation, EnvIsolation: &envIsolation},
		Limits:    Limits{MaxIterations: &iterations, GrindCost: &grindCost, GrindTokens: &grindTokens, GrindContinuations: &grindContinuations, MaxConcurrency: &maxConcurrency, Compaction: defaultCompaction()},
		Reasoning: Reasoning{Thinking: &thinking, Budget: &budget},
		Discovery: Discovery{
			ProjectContext: &projectContext,
			Skills:         &skills,
			TrustSkills:    &trustSkills,
			TrustHooks:     &trustHooks,
		},
		UI:      UI{Mode: &mode, GroupTools: &groupTools, ShowThinking: &showThinking, Diffs: &diffs, PromptPlaceholder: &promptPlaceholder, StartMessage: &startMessage, TransparentBlocks: &transparent, JSON: &json, ShowHooks: &showHooks, ShowHookOutput: &showHookOutput},
		Editor:  Editor{Editor: "", PromptEditKey: "ctrl+g"},
		Sandbox: Sandbox{VMName: "", Port: 2226, SSHKeyPath: "~/.ssh/id_ed25519", AutoSnapshot: &autoSnapshot},
		Remote:  Remote{},
		Chat:    Chat{Matrix: Matrix{Enabled: &chatOff}},
	}
}
