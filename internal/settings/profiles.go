package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Profile defines a reusable identity: the provider and reasoning that reach a
// backend, the limits that tune a run to that model, and the persona text the
// session layers on top of the base prompt. Behaviour — tools, security,
// sources, gates — stays in ~/.bulle.yml, so one profile works in every
// checkout without carrying that checkout's environment along.
type Profile struct {
	Name             string    `yaml:"name"`
	Provider         Provider  `yaml:"provider"`
	Reasoning        Reasoning `yaml:"reasoning"`
	Limits           Limits    `yaml:"limits"`
	AdditionalPrompt string    `yaml:"additional_prompt"`
}

// ProfilesDir returns the path to ~/.bulle/profiles.
func ProfilesDir() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles"), nil
}

// LoadProfiles reads every .yml file in dir as one Profile, sorted by name.
func LoadProfiles(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	seen := make(map[string]string, len(entries))
	profiles := make([]Profile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), ".yml")
		p, err := loadProfileFile(filepath.Join(dir, entry.Name()), stem)
		if err != nil {
			return nil, err
		}
		if first, clash := seen[p.Name]; clash {
			return nil, fmt.Errorf("duplicate profile name %q: %s and %s both define it", p.Name, first, filepath.Join(dir, entry.Name()))
		}
		seen[p.Name] = filepath.Join(dir, entry.Name())
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

func loadProfileFile(path string, stem string) (Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("reading %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var p Profile
	if err := decoder.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return Profile{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if p.Name == "" {
		p.Name = stem
	}
	return p, nil
}

// FindProfile returns the profile with the given name from a slice.
func FindProfile(profiles []Profile, name string) (Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// ApplyProfile overwrites the provider, reasoning and limit fields the profile
// actually sets, and layers its persona text on top of the prompt. A profile is
// a layer of its own, above ~/.bulle.yml: it moves the identity it names even
// when the file names one too, and only the environment and the flags, merged
// after it, can still win a field back.
func ApplyProfile(c *Config, p Profile) {
	if p.Provider.Backend != "" {
		c.Backend = p.Provider.Backend
	}
	if p.Provider.Model != "" {
		c.Model = p.Provider.Model
	}
	if p.Provider.BaseURL != "" {
		c.BaseURL = p.Provider.BaseURL
	}
	if p.Provider.APIKey != "" {
		c.APIKey = p.Provider.APIKey
	}
	if p.Provider.APIKeyCommand != "" {
		c.APIKeyCommand = p.Provider.APIKeyCommand
	}
	if p.Reasoning.Effort != "" {
		c.Effort = p.Reasoning.Effort
	}
	if p.Reasoning.Thinking != nil {
		c.Thinking = p.Reasoning.Thinking
	}
	if p.Reasoning.Budget != nil {
		c.Budget = p.Reasoning.Budget
	}
	c.mergeLimits(Config{Limits: p.Limits})
	if p.AdditionalPrompt != "" {
		c.Additional = p.AdditionalPrompt
	}
}

// selectedProfile is the profile the layers name: the flag beats the
// environment beats the file, the same order every other setting follows.
// Reading the name out before any layer is merged is what lets the profile be
// applied as a layer of its own rather than at whichever layer named it — see
// resolveLayers, and the asymmetry that used to make the same profile mean two
// different things depending on how it was selected.
func selectedProfile(file, env, flags Config) string {
	for _, layer := range []Config{flags, env, file} {
		if layer.Profile != "" {
			return layer.Profile
		}
	}
	return ""
}

// ResolveProfile loads and applies the named profile onto the Config.
func ResolveProfile(c *Config, name string) error {
	if name == "" {
		return nil
	}
	dir, err := ProfilesDir()
	if err != nil {
		return err
	}
	profiles, err := LoadProfiles(dir)
	if err != nil {
		return err
	}
	p, ok := FindProfile(profiles, name)
	if !ok {
		return fmt.Errorf("unknown profile %q: not found in %s", name, dir)
	}
	ApplyProfile(c, p)
	c.Profile = name
	return nil
}
