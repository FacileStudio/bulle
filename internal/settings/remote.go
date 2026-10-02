package settings

// RemoteTarget is one configured SSH host under remote.targets. Host is the
// only required field: an entry that sets just host uses the group's user,
// port, identity and workspace. A target whose key is itself an address can
// leave host empty and let the key resolve.
type RemoteTarget struct {
	Host       string `json:"host" yaml:"host"`
	Port       int    `json:"port" yaml:"port"`
	User       string `json:"user" yaml:"user"`
	SSHKeyPath string `json:"ssh_key_path" yaml:"ssh_key_path"`
	Root       string `json:"root" yaml:"root"`
}

// Remote holds the defaults `bulle remote` applies to SSH hosts: the host it
// connects to with no argument, and the user, port, identity and workspace
// every host inherits unless it overrides them. An empty port is meaningful —
// it lets ssh pick, honoring ~/.ssh/config's Port before ssh's own default of
// 22 — so the group leaves it at zero rather than spelling 22 and overriding an
// alias. An empty root means the SSH login directory, which is the user's home
// — where a bare ssh lands — rather than a path guessed here. It is a separate
// group from sandbox because the two name different machines — a boite microVM
// on loopback versus a host reached over the network — and a shared group would
// have to spell a port default that is wrong for one of them.
type Remote struct {
	Default    string                  `json:"default" yaml:"default"`
	User       string                  `json:"user" yaml:"user"`
	Port       int                     `json:"port" yaml:"port"`
	SSHKeyPath string                  `json:"ssh_key_path" yaml:"ssh_key_path"`
	Root       string                  `json:"root" yaml:"root"`
	Targets    map[string]RemoteTarget `json:"targets" yaml:"targets"`
}

func (t *RemoteTarget) merge(over RemoteTarget) {
	if over.Host != "" {
		t.Host = over.Host
	}
	if over.Port != 0 {
		t.Port = over.Port
	}
	if over.User != "" {
		t.User = over.User
	}
	if over.SSHKeyPath != "" {
		t.SSHKeyPath = over.SSHKeyPath
	}
	if over.Root != "" {
		t.Root = over.Root
	}
}

func (r *Remote) mergeBasic(over Remote) {
	if over.Default != "" {
		r.Default = over.Default
	}
	if over.User != "" {
		r.User = over.User
	}
	if over.Port != 0 {
		r.Port = over.Port
	}
	if over.SSHKeyPath != "" {
		r.SSHKeyPath = over.SSHKeyPath
	}
	if over.Root != "" {
		r.Root = over.Root
	}
}

func (r *Remote) mergeTargets(over map[string]RemoteTarget) {
	if len(over) == 0 {
		return
	}
	if r.Targets == nil {
		r.Targets = make(map[string]RemoteTarget, len(over))
	}
	for k, v := range over {
		existing, ok := r.Targets[k]
		if !ok {
			r.Targets[k] = v
			continue
		}
		existing.merge(v)
		r.Targets[k] = existing
	}
}

func (r *Remote) merge(over Remote) {
	r.mergeBasic(over)
	r.mergeTargets(over.Targets)
}
