package settings

import (
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"sync"

	"go.yaml.in/yaml/v4"
)

// pathList collects one -skill-dir or -mcp flag per occurrence.
type pathList []string

func (p *pathList) String() string     { return strings.Join(*p, ":") }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

// declared holds every flag pointer returned by declareFlags.
type declared struct {
	backend, model, root, system, additional, profile *string
	reasoningFlags
	togglesFlags
	iterations *int
	// compactAt is a token count, not a turn count, so it shares the width of
	// budget rather than iterations.
	compactAt         *int64
	maxConcurrency    *int
	maxParallelAgents *int
	uiFlags
	sourceFlags
	discoveryFlags
}

// uiFlags is the -continue, -resume, -mode, -transparent-blocks and -json
// switches. Grouped together so declared stays under filet's struct cap;
// continue auto-resumes the newest session, resume names one by id or path,
// mode picks between "inline" and "tui" rendering, transparent-blocks drops
// the pane backdrop tool results sit on, and json makes cron list print one
// JSON document.
type uiFlags struct {
	cont           *bool
	resume         *string
	mode           *string
	transparent    *bool
	json           *bool
	noConfig       *bool
	showHooks      *bool
	showHookOutput *bool
}

type togglesFlags struct {
	bash, parallelAgents, fetch, approveTools, diffs, tasks, diagnostics, searchContent, findFiles *bool
}

type reasoningFlags struct {
	effort   *string
	thinking *bool
	budget   *int64
}

type sourceFlags struct {
	skillDirs, mcp *pathList
	gatesFile      *string
}

type discoveryFlags struct {
	projectContext, skills, trustSkills, trustHooks *bool
}

// declareFlags registers every flag against fallback's values and returns
// the pointers flag.Parse will fill in.
func declareFlags(fallback Config) declared {
	return declared{
		sourceFlags: declareSources(fallback),
		backend:     flag.String("backend", fallback.Backend, "anthropic, google, openai, or openrouter"),
		model:       flag.String("model", fallback.Model, "model id, defaulting to the backend's own"),
		profile:     flag.String("profile", fallback.Profile, "profile name from ~/.bulle/profiles/"),
		root:        flag.String("root", fallback.Root, "directory the file tools may reach"),
		system:      flag.String("system-prompt", fallback.System, "system prompt"),
		additional:  flag.String("additional-prompt", fallback.Additional, "text appended after the base system prompt"),
		uiFlags: uiFlags{
			cont:           flag.Bool("continue", *fallback.Continue, "auto-resume newest session"),
			resume:         flag.String("resume", *fallback.Resume, "resume a specific session by id or file path"),
			mode:           flag.String("mode", *fallback.Mode, "inline or tui rendering"),
			transparent:    flag.Bool("transparent-blocks", *fallback.TransparentBlocks, "drop the backdrop on tool result and diff panes"),
			json:           flag.Bool("json", *fallback.JSON, "print cron list as one JSON document"),
			noConfig:       flag.Bool("no-config", false, "start with default settings, ignoring ~/.bulle.yml"),
			showHooks:      flag.Bool("show-hooks", *fallback.ShowHooks, "show hook execution in conversation"),
			showHookOutput: flag.Bool("show-hook-output", *fallback.ShowHookOutput, "show hook output preview in conversation"),
		},
		reasoningFlags: reasoningFlags{
			effort:   flag.String("effort", fallback.Effort, "none, minimal, low, medium, high, xhigh or max"),
			thinking: flag.Bool("thinking", *fallback.Thinking, "stream the model's reasoning"),
			budget:   flag.Int64("reasoning-budget", *fallback.Budget, "tokens one turn may spend on reasoning; 0 sets no ceiling"),
		},
		togglesFlags:      declareToggles(fallback),
		iterations:        flag.Int("max-iterations", *fallback.MaxIterations, "how many times the model may be asked"),
		compactAt:         flag.Int64("compact-at", 0, "absolute transcript size in tokens at which the session compacts; 0 turns compaction off, leaving it unset derives it from the context window"),
		maxConcurrency:    flag.Int("max-concurrency", 16, "maximum concurrent workers"),
		maxParallelAgents: flag.Int("max-parallel-agents", 16, "maximum parallel agents"),
		discoveryFlags: discoveryFlags{
			projectContext: flag.Bool("project-context", *fallback.ProjectContext, "read CLAUDE.md and AGENTS.md from root upward into the system prompt"),
			skills:         flag.Bool("skills", *fallback.Skills, "tell the model about skills found in ~/.agents/skills and trusted .agents/skills directories"),
			trustSkills:    flag.Bool("trust-skills", *fallback.TrustSkills, "trust every .agents/skills directory found under root this run, and remember the decision"),
			trustHooks:     flag.Bool("trust-hooks", *fallback.TrustHooks, "trust this project's .bulle/hooks.yml as it reads right now, and remember that version"),
		},
	}
}

func declareToggles(fallback Config) togglesFlags {
	return togglesFlags{
		bash:           flag.Bool("bash", *fallback.Bash, "let the model run commands"),
		parallelAgents: flag.Bool("parallel-agents", *fallback.ParallelAgents, "give the model a parallel delegate tool that fans independent tasks out to concurrent nested runs; on by default"),
		fetch:          flag.Bool("fetch", *fallback.Fetch, "let the model read a web page by URL; on by default"),
		approveTools:   flag.Bool("approve-tools", *fallback.ApproveTools, "ask before every tool call runs, y/a/n; off by default, every call runs unasked"),
		diffs:          flag.Bool("diffs", *fallback.Diffs, "show a git-style diff when the model edits a file; on by default"),
		tasks:          flag.Bool("tasks", *fallback.Tasks, "give the model a task planning tool to create and update checklists; on by default"),
		diagnostics:    flag.Bool("diagnostics", *fallback.Diagnostics, "give the model post-edit diagnostics and a tool to pull them; on by default"),
		searchContent:  flag.Bool("search-content", *fallback.SearchContent, "let the model search file contents with regex; on by default"),
		findFiles:      flag.Bool("find-files", *fallback.FindFiles, "let the model find files by glob; on by default"),
	}
}

func declareSources(fallback Config) sourceFlags {
	skillDirs, mcp := new(pathList), new(pathList)
	flag.Var(skillDirs, "skill-dir",
		"extra directory to load skills from, alongside ~/.agents/skills (repeatable)")
	flag.Var(mcp, "mcp",
		"file of MCP servers to start and hand the model the tools of (repeatable)")
	gatesFile := flag.String("gates-file", fallback.GatesFile,
		"file of gate checks the session must pass, overriding the main settings' gates")
	return sourceFlags{skillDirs: skillDirs, mcp: mcp, gatesFile: gatesFile}
}

// loadGatesFile reads a --gates-file: the same gates block the main settings
// carry, in a file of its own. Naming one is a deliberate act, so a missing or
// malformed file is an error rather than the silent fall-through an absent
// config file gets.
func loadGatesFile(path string) ([]GateSpec, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}
	defer func() { _ = file.Close() }()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var parsed struct {
		Gates []GateSpec `yaml:"gates"`
	}
	if err := decoder.Decode(&parsed); err != nil && !errors.Is(err, io.EOF) {
		return nil, &ParseError{Path: path, Err: err}
	}
	return parsed.Gates, nil
}

// resolveGates lets a named gates file replace the chain the lower layers
// resolved, then validates whatever gates the config carries: the flag
// carries a whole file of gates, so it speaks last.
func resolveGates(resolved Config, path string) (Config, error) {
	if path != "" {
		gates, err := loadGatesFile(path)
		if err != nil {
			return Config{}, err
		}
		resolved.Gates = gates
	}
	for _, g := range resolved.Gates {
		if g.Name == "" || len(g.Command) == 0 {
			return Config{}, &ParseError{Path: "gates", Err: errors.New("every gate needs a name and a command")}
		}
	}
	return resolved, nil
}

// processFlags declares bulle's flags on the process's command line the first
// time it is asked and returns that same declaration every time after: the flag
// package holds one CommandLine for the process and panics when a name is
// declared on it twice, which is what a test binary running these tests twice —
// go test -count=2 — did. The first caller's fallback seeds the flag defaults.
var processFlags = func() func(Config) declared {
	var once sync.Once
	var first declared
	return func(fallback Config) declared {
		once.Do(func() { first = declareFlags(fallback) })
		return first
	}
}()

// FromFlags is the settings layer the command line supplies.
//
// Only the flags actually typed are collected. It calls flag.Parse internally.
func FromFlags(fallback Config) Config {
	f := processFlags(fallback)
	flag.Parse()
	typed := typedSetters(f)

	var flags Config
	flag.Visit(func(flg *flag.Flag) {
		if take, known := typed[flg.Name]; known {
			take(&flags)
		}
	})
	return flags
}
