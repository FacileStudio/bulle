package settings

import (
	"fmt"
	"os"
)

// Template is the scaffold written to ~/.bulle.yml on first boot: only the
// settings a first run has to choose, every value either empty or the shipped
// default. The point is not discoverability — that is docs/configuration.md, the
// full reference — but to hand a newcomer the four real decisions (backend,
// session root, the judge, and compact_at) with enough comment to edit from,
// without freezing the other defaults into a file nobody will read past line
// one.
//
// The rest of the surface is documented in docs/configuration.md and stays
// honoured from old files: struct fields, merge rules and env reads are all
// untouched, so a two-hundred-line ~/.bulle.yml from an earlier release still
// parses and still takes effect. Keys that have left the scaffold are sunstopped
// in docs, not removed from code.
//
// example.bulle.yml in the repo root is this same text, byte for byte, and
// TestExampleConfigIsTheScaffoldTemplate fails when the two drift apart: the
// README promises the example is what a first boot writes.
const Template = `# bulle settings — written on first boot, edited from here.
# Delete this file and bulle writes it again. Every setting is documented in
# docs/configuration.md; this file holds only the four choices a first run
# actually has to make, with the rest left to defaults that are right for most
# sessions.
provider:
  backend: anthropic
  model: ""
  base_url: ""
  api_key: ""
  # api_key_command is run to obtain the key when api_key is empty, so this file
  # can carry no secret: "tiroir get ANTHROPIC_API_KEY", "op read op://v/k".
  # The command prints the key on stdout. A key from a flag, the environment,
  # a profile or another file wins; the command is refused at startup, not
  # ignored, when it fails.
  # api_key_command: ""

session:
  root: .

limits:
  max_iterations: 5
  max_concurrency: 16
  # compact_at is an absolute token ceiling. Left unset (the default) the
  # ceiling comes from the backend's context window and the compaction ratios;
  # 0 turns compaction off outright.
  # compact_at: 75000
  # The compaction judge is opt-in and off by default. Enabling it sends
  # conversation history — which can include source code and secrets — to a
  # third-party decision model. provider names the model: "jev" (TypeSafe
  # System One, the default) or "clef" (Cloudflare via OpenRouter). An explicit
  # model/base_url still wins when set, as an escape hatch rather than a
  # supported choice. The key prefers TYPESAFE_API_KEY over judge.api_key.
  compaction:
    judge:
      enabled: false
      provider: jev
      # model, base_url, api_key and prune_threshold are internalised defaults:
      # they are read from the file when present but no longer scaffolded or
      # documented. Leave them out unless you know you need them.
      # api_key_command: tiroir get TYPESAFE_API_KEY

security:
  # deny_elevation refuses sudo-style elevation in run_command: a policy guard
  # against accidents and injected instructions, not a security boundary.
  # OS-level enforcement is the real wall.
  deny_elevation: true
  env_isolation: false

editor:
  # empty uses the system EDITOR; set a path to force one (e.g. /usr/bin/vim)
  # editor: ""
  # prompt_edit_key: ctrl+g # opens the external editor on the prompt

# tools, discovery, ui, editor, sources, hooks, gates, sandbox, remote and the
# rest are all in docs/configuration.md. The only ones worth naming here at first
# boot are the ones above: the provider, the session root, the judge, and
# compact_at. Everything else ships a default that is right for most sessions.
`

// Scaffold writes the template when no config file exists yet, and reports
// whether it did. An existing file is never touched: the file is the user's
// answer to "what do I want", not a cache. A parse error in an existing file
// is surfaced by Load, not papered over here.
func Scaffold(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stating %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(Template), 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, nil
}
