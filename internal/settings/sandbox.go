package settings

import (
	"errors"
	"strings"

	"go.yaml.in/yaml/v4"
)

// SandboxTarget is one configured boite microVM under sandbox.targets. The key
// names the target, and vm_name names the boite instance behind it when the two
// differ; the rest are the overrides that belong to that one VM.
//
// There is no backend or host field: sandbox targets are boite by definition,
// and SSH hosts live under remote.targets. Binary synchronization is not a
// setting either — bulle runs on the host and only tool calls cross the boundary.
type SandboxTarget struct {
	VMName       string `json:"vm_name" yaml:"vm_name"`
	User         string `json:"user" yaml:"user"`
	Port         int    `json:"port" yaml:"port"`
	SSHKeyPath   string `json:"ssh_key_path" yaml:"ssh_key_path"`
	Root         string `json:"root" yaml:"root"`
	AutoSnapshot *bool  `json:"auto_snapshot" yaml:"auto_snapshot"`
}

// Sandbox holds the boite microVM defaults: which VM `bulle sandbox` enters
// with no argument, and the SSH identity and workspace it uses unless a target
// overrides them. An empty root means the instance's own workspace, where boite
// mounted the project, so no path is guessed when the config says nothing.
type Sandbox struct {
	Default      string                   `json:"default" yaml:"default"`
	User         string                   `json:"user" yaml:"user"`
	Targets      map[string]SandboxTarget `json:"targets" yaml:"targets"`
	VMName       string                   `json:"vm_name" yaml:"vm_name"`
	Port         int                      `json:"port" yaml:"port"`
	SSHKeyPath   string                   `json:"ssh_key_path" yaml:"ssh_key_path"`
	Root         string                   `json:"root" yaml:"root"`
	AutoSnapshot *bool                    `json:"auto_snapshot" yaml:"auto_snapshot"`
}

func (t *SandboxTarget) merge(over SandboxTarget) {
	if over.VMName != "" {
		t.VMName = over.VMName
	}
	if over.User != "" {
		t.User = over.User
	}
	if over.Port != 0 {
		t.Port = over.Port
	}
	if over.SSHKeyPath != "" {
		t.SSHKeyPath = over.SSHKeyPath
	}
	if over.Root != "" {
		t.Root = over.Root
	}
	mergeBool(&t.AutoSnapshot, over.AutoSnapshot)
}

func (s *Sandbox) mergeBasic(over Sandbox) {
	if over.Default != "" {
		s.Default = over.Default
	}
	if over.User != "" {
		s.User = over.User
	}
	if over.VMName != "" {
		s.VMName = over.VMName
	}
	if over.Port != 0 {
		s.Port = over.Port
	}
	if over.SSHKeyPath != "" {
		s.SSHKeyPath = over.SSHKeyPath
	}
	if over.Root != "" {
		s.Root = over.Root
	}
	mergeBool(&s.AutoSnapshot, over.AutoSnapshot)
}

func (s *Sandbox) mergeTargets(over map[string]SandboxTarget) {
	if len(over) == 0 {
		return
	}
	if s.Targets == nil {
		s.Targets = make(map[string]SandboxTarget, len(over))
	}
	for k, v := range over {
		existing, ok := s.Targets[k]
		if !ok {
			s.Targets[k] = v
			continue
		}
		existing.merge(v)
		s.Targets[k] = existing
	}
}

func (s *Sandbox) merge(over Sandbox) {
	s.mergeBasic(over)
	s.mergeTargets(over.Targets)
}

// repointSandboxKeys rewrites the strict refusal of a setting the sandbox group
// no longer has — an SSH-shaped target, workdir or auto_sync — so a config
// written before the split says where the setting went instead of only refusing
// it. Every other unknown field keeps the decoder's own message.
func repointSandboxKeys(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	rewritten := make([]*yaml.UnmarshalError, 0, len(typeErr.Errors))
	for _, item := range typeErr.Errors {
		item = &yaml.UnmarshalError{Err: repointSandboxMessage(item.Err.Error()), Line: item.Line, Column: item.Column}
		rewritten = append(rewritten, item)
	}
	return &yaml.TypeError{Errors: rewritten}
}

// repointSandboxMessage names the new home for a removed sandbox key, and
// returns the decoder's message unchanged for anything else.
//
// It matches on the decoder's own wording ("field X not found"), so a yaml v4
// release that rewords that message silently drops the guidance back to the raw
// error. That is a soft failure — the config is still refused, loudly — but it
// is why TestSandboxRemovedKeysRepointed pins the message rather than only the
// error type.
func repointSandboxMessage(msg string) error {
	switch {
	case strings.Contains(msg, "field backend not found"), strings.Contains(msg, "field host not found"):
		return errors.New("sandbox targets are boite-only; an SSH host belongs under remote.targets")
	case strings.Contains(msg, "field workdir not found"):
		return errors.New("sandbox target workdir is now root")
	case strings.Contains(msg, "field auto_sync not found"):
		return errors.New("auto_sync is gone: bulle runs on the host and syncs no binary into the target")
	}
	return errors.New(msg)
}
