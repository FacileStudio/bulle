package settings

// Automation groups the config's chained machinery. It is inline in the
// YAML, so the keys stay top-level: hooks, gates, assistant. Scheduled jobs are
// not here anymore: LoadJobs reads them from ~/.config/bulle/jobs/ instead.
type Automation struct {
	Hooks     []HookSpec `yaml:"hooks"`
	Gates     []GateSpec `yaml:"gates"`
	Assistant Assistant  `yaml:"assistant"`
}

// Assistant configures the proactive assistant: when it speaks, what it reads
// and who may always interrupt. Every empty value falls back to the default
// the plan fixes, so a config that never mentions assistant: still works.
type Assistant struct {
	Timezone       string            `json:"timezone,omitempty" yaml:"timezone"`
	Digests        map[string]string `json:"digests,omitempty" yaml:"digests"`
	QuietStart     string            `json:"quiet_start,omitempty" yaml:"quiet_start"`
	QuietEnd       string            `json:"quiet_end,omitempty" yaml:"quiet_end"`
	WeekendDigests []string          `json:"weekend_digests,omitempty" yaml:"weekend_digests"`
	Profile        string            `json:"profile,omitempty" yaml:"profile"`
	Language       string            `json:"language,omitempty" yaml:"language"`
	Sources        AssistantSources  `json:"sources" yaml:"sources"`
	VIP            []string          `json:"vip,omitempty" yaml:"vip"`
	DropSenders    []string          `json:"drop_senders,omitempty" yaml:"drop_senders"`
	Voice          AssistantVoice    `json:"voice" yaml:"voice"`
}

// AssistantSources lists the Facile accounts, calendars and workspaces the
// assistant reads. An empty list means every one the CLI reports.
type AssistantSources struct {
	MailAccounts []int `json:"mail_accounts,omitempty" yaml:"mail_accounts"`
	Calendars    []int `json:"calendars,omitempty" yaml:"calendars"`
	Workspaces   []int `json:"workspaces,omitempty" yaml:"workspaces"`
}

// AssistantVoice is the speech-to-text command. {wav} in Command is replaced
// with the path of a 16 kHz mono wav file, and the transcript is read from
// the command's standard output.
type AssistantVoice struct {
	Command string `json:"command,omitempty" yaml:"command"`
}

const (
	DefaultAssistantTimezone   = "Europe/Paris"
	DefaultAssistantQuietStart = "22:00"
	DefaultAssistantQuietEnd   = "08:00"
	DefaultAssistantLanguage   = "mirror"
)

// DefaultAssistantDigests are the three daily digests, name to local time.
func DefaultAssistantDigests() map[string]string {
	return map[string]string{"morning": "09:00", "midday": "14:00", "evening": "20:00"}
}

func (a *Assistant) merge(over Assistant) {
	fill(&a.Timezone, over.Timezone)
	fill(&a.QuietStart, over.QuietStart)
	fill(&a.QuietEnd, over.QuietEnd)
	fill(&a.Profile, over.Profile)
	fill(&a.Language, over.Language)
	fill(&a.Voice.Command, over.Voice.Command)
	if len(over.Digests) > 0 {
		a.Digests = over.Digests
	}
	if len(over.WeekendDigests) > 0 {
		a.WeekendDigests = over.WeekendDigests
	}
	if len(over.VIP) > 0 {
		a.VIP = over.VIP
	}
	if len(over.DropSenders) > 0 {
		a.DropSenders = over.DropSenders
	}
	a.Sources.merge(over.Sources)
}

func (s *AssistantSources) merge(over AssistantSources) {
	if len(over.MailAccounts) > 0 {
		s.MailAccounts = over.MailAccounts
	}
	if len(over.Calendars) > 0 {
		s.Calendars = over.Calendars
	}
	if len(over.Workspaces) > 0 {
		s.Workspaces = over.Workspaces
	}
}

// WithDefaults returns a copy with every empty field filled from the defaults.
func (a Assistant) WithDefaults() Assistant {
	fill(&a.Timezone, DefaultAssistantTimezone)
	fill(&a.QuietStart, DefaultAssistantQuietStart)
	fill(&a.QuietEnd, DefaultAssistantQuietEnd)
	fill(&a.Language, DefaultAssistantLanguage)
	if len(a.Digests) == 0 {
		a.Digests = DefaultAssistantDigests()
	}
	if len(a.WeekendDigests) == 0 {
		a.WeekendDigests = []string{"morning"}
	}
	return a
}
