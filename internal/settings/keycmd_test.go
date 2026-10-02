package settings

import (
	"strings"
	"testing"
)

// keyPrecedenceCases are the layers a key command competes with. Every case
// would pass just as well if the command always ran and its answer was thrown
// away, so the ones a command loses name a command that fails loudly — a run
// that ignores precedence reports that failure rather than quietly preferring
// the key it was handed.
var keyPrecedenceCases = []struct {
	name string
	file string
	env  map[string]string
	want string
}{
	{
		name: "the command fills a key the file left out",
		file: "provider:\n  api_key_command: \"echo sk-from-command\"\n",
		want: "sk-from-command",
	},
	{
		name: "a literal key beats its own command",
		file: "provider:\n  api_key: sk-literal\n  api_key_command: \"exit 3\"\n",
		want: "sk-literal",
	},
	{
		name: "the environment beats the command",
		file: "provider:\n  api_key_command: \"exit 3\"\n",
		env:  map[string]string{"BULLE_PROVIDER_API_KEY": "sk-from-env"},
		want: "sk-from-env",
	},
	{
		name: "no command and no key stays empty",
		file: "provider:\n  model: claude-opus-5\n",
		want: "",
	},
}

// keyFailureCases are the ways a command can fail to produce a key. Each is a
// load error naming the field it was written in, never an empty key: an empty
// key reaches the backend as "no credential" and reads exactly like a config
// that forgot one, which is the failure this setting exists to make impossible.
var keyFailureCases = []struct {
	name string
	file string
	want string
}{
	{
		name: "a provider command that fails",
		file: "provider:\n  api_key_command: \"exit 3\"\n",
		want: "provider.api_key_command",
	},
	{
		name: "a provider command that prints nothing",
		file: "provider:\n  api_key_command: \"true\"\n",
		want: "provider.api_key_command",
	},
	{
		name: "a judge command that fails",
		file: "limits:\n  compaction:\n    judge:\n      api_key_command: \"exit 3\"\n",
		want: "limits.compaction.judge.api_key_command",
	},
}

// A key command is a source, not an override: it fills a key nothing else
// supplied, and every layer that did supply one wins.
func TestResolveKeysPrecedence(t *testing.T) {
	clearEnv(t, "BULLE_PROVIDER_API_KEY", "NACELLE_PROVIDER_API_KEY", "TYPESAFE_API_KEY")
	for _, tt := range keyPrecedenceCases {
		t.Run(tt.name, func(t *testing.T) {
			written(t, tt.file)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}
			cfg, err := settings(Config{})
			if err != nil {
				t.Fatalf("settings: %v", err)
			}
			if err := ResolveKeys(&cfg); err != nil {
				t.Fatalf("ResolveKeys: %v", err)
			}
			if cfg.APIKey != tt.want {
				t.Errorf("api key did not come out as %q", tt.want)
			}
		})
	}
}

// The file's command reaching the loader is only half of it: the value it prints
// has to land where the session reads it, and the judge has its own field and its
// own command, so one key can be a literal while the other is fetched.
func TestKeyCommandsReachTheirOwnField(t *testing.T) {
	clearEnv(t, "TYPESAFE_API_KEY", "BULLE_COMPACTION_JUDGE_API_KEY", "NACELLE_COMPACTION_JUDGE_API_KEY")
	written(t, `provider:
  api_key_command: "echo sk-provider"
limits:
  compaction:
    judge:
      enabled: true
      api_key_command: "echo sk-judge"
`)
	cfg, err := settings(Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if err := ResolveKeys(&cfg); err != nil {
		t.Fatalf("ResolveKeys: %v", err)
	}
	if cfg.APIKey != "sk-provider" {
		t.Error("the provider key is not the provider command's answer")
	}
	if got := cfg.Compaction.Judge.APIKey; got != "sk-judge" {
		t.Error("the judge key is not the judge command's answer")
	}
}

// A profile is where this setting earns its keep — one file per identity, none
// of them carrying a secret — so the profile's own command has to survive the
// layer that applies it, over a file that names no command at all.
func TestAProfileKeyCommandReachesTheResolvedConfig(t *testing.T) {
	clearEnv(t, "BULLE_PROVIDER_API_KEY", "NACELLE_PROVIDER_API_KEY", "TYPESAFE_API_KEY")
	setupProfileEnvWith(t, "lerouteur.yml",
		"name: lerouteur\nprovider:\n  backend: openrouter\n  model: deepseek\n  api_key_command: \"echo sk-profile\"\n",
		"profile: lerouteur\n")

	cfg, err := Settings("", Config{})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if cfg.APIKeyCommand != "echo sk-profile" {
		t.Fatalf("api_key_command = %q, want the profile's command to survive the layer", cfg.APIKeyCommand)
	}
	if err := ResolveKeys(&cfg); err != nil {
		t.Fatalf("ResolveKeys: %v", err)
	}
	if cfg.APIKey != "sk-profile" {
		t.Error("the profile's api key is not its command's answer")
	}
}

// A command that cannot produce a key is refused at load, naming where it was
// written, rather than leaving a credential-shaped hole behind.
func TestAFailingKeyCommandNamesItsField(t *testing.T) {
	clearEnv(t, "TYPESAFE_API_KEY", "BULLE_COMPACTION_JUDGE_API_KEY", "NACELLE_COMPACTION_JUDGE_API_KEY")
	for _, tt := range keyFailureCases {
		t.Run(tt.name, func(t *testing.T) {
			written(t, tt.file)
			cfg, err := settings(Config{})
			if err != nil {
				t.Fatalf("settings: %v", err)
			}
			err = ResolveKeys(&cfg)
			if err == nil {
				t.Fatal("a command that produces no key was accepted, want a load error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to name %s", err, tt.want)
			}
		})
	}
}

// The command is a shell program, not a name and an argument list: `tiroir
// unlock && tiroir get K` and a pipeline both have to work, and a `get` that
// prints its answer with a trailing newline is the ordinary case rather than one
// to special-case at every call site.
func TestKeyFromCommandRunsThroughAShell(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "no command is no key", command: "", want: ""},
		{name: "the trailing newline is not part of the key", command: "echo sk-newline", want: "sk-newline"},
		{name: "a pipeline runs as written", command: "printf sk-piped | tr a-z A-Z", want: "SK-PIPED"},
		{name: "two commands in sequence", command: "true && echo sk-chained", want: "sk-chained"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KeyFromCommand(tt.command)
			if err != nil {
				t.Fatalf("KeyFromCommand(%q): %v", tt.command, err)
			}
			if got != tt.want {
				t.Errorf("KeyFromCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
