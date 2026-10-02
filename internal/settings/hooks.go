package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/FacileStudio/nacelle"
	"go.yaml.in/yaml/v4"
)

// HooksFile is the project-level hooks file, read in addition to the
// `hooks:` entries in the user's own config. The legacy .nacelle/ name is
// still read when the new one is absent, so a project configured before the
// rename keeps its hooks.
const HooksFile = ".bulle/hooks.yml"

const legacyHooksFile = ".nacelle/hooks.yml"

// parseHooks decodes one hooks file.
func parseHooks(raw []byte) ([]HookSpec, error) {
	var file struct {
		Hooks []HookSpec `yaml:"hooks"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return file.Hooks, nil
}

// BuildHooks turns config entries into live library hooks, refusing any
// spec that would otherwise fail silently mid-session.
func BuildHooks(specs []HookSpec) (map[nacelle.HookPoint][]nacelle.Hook, error) {
	var hooks map[nacelle.HookPoint][]nacelle.Hook
	for _, spec := range specs {
		if err := spec.Validate(); err != nil {
			return nil, err
		}
		var hook nacelle.Hook
		if spec.Async {
			hook = nacelle.Async(execHook(spec))
		} else {
			hook = nacelle.WithTimeout(spec.Duration(), execHook(spec))
		}
		hooks = appendHook(hooks, HookPointOf(spec.On), hook)
	}
	return hooks, nil
}

func appendHook(hooks map[nacelle.HookPoint][]nacelle.Hook, p nacelle.HookPoint, hook nacelle.Hook) map[nacelle.HookPoint][]nacelle.Hook {
	if hooks == nil {
		hooks = map[nacelle.HookPoint][]nacelle.Hook{}
	}
	hooks[p] = append(hooks[p], hook)
	return hooks
}

// hookPayload is the process contract's input: one JSON object on stdin.
type hookPayload struct {
	Event  string `json:"event"`
	Tool   string `json:"tool"`
	Input  string `json:"input"`
	Result string `json:"result,omitempty"`
	Retry  bool   `json:"retry"`
}

// SessionHooks resolves every hooks layer in one place.
func SessionHooks(config Config) (map[nacelle.HookPoint][]nacelle.Hook, string, error) {
	hooks, err := BuildHooks(config.Hooks)
	if err != nil {
		return nil, "", err
	}

	project, notice, err := LoadProjectHooks(config.Root, *config.TrustHooks)
	if err != nil {
		return nil, "", err
	}
	for p, list := range project {
		for _, hook := range list {
			hooks = appendHook(hooks, p, hook)
		}
	}
	return hooks, notice, nil
}

// LoadProjectHooks reads <root>/.bulle/hooks.yml through the trust gate,
// falling back to the legacy .nacelle/hooks.yml.
func LoadProjectHooks(root string, trustNew bool) (map[nacelle.HookPoint][]nacelle.Hook, string, error) {
	path := filepath.Join(root, HooksFile)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		path = filepath.Join(root, legacyHooksFile)
		raw, err = os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil, "", nil
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", path, err)
	}

	specs, err := parseHooks(raw)
	if err != nil {
		return nil, "", fmt.Errorf("parsing %s: %w", path, err)
	}
	hooks, err := BuildHooks(specs)
	if err != nil {
		return nil, "", fmt.Errorf("in %s: %w", path, err)
	}

	trusted, err := hookIsTrusted(path, raw, trustNew)
	if err != nil {
		return nil, "", err
	}
	if !trusted {
		return nil, fmt.Sprintf(
			"This project defines hooks in %s (%d lines of commands that run on every tool call) and they are not trusted yet.\n"+
				"Read them, then restart with -trust-hooks to approve this version.", HooksFile, bytes.Count(raw, []byte("\n"))), nil
	}
	return hooks, "", nil
}

// hookIsTrusted reports whether this exact hooks file content has been
// approved, recording the approval when the session runs with -trust-hooks.
func hookIsTrusted(path string, raw []byte, trustNew bool) (bool, error) {
	trusted, err := IsTrusted(path, raw)
	if err != nil || trusted || !trustNew {
		return trusted, err
	}
	return true, Save(path, raw)
}
