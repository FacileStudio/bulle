package settings

import (
	"path/filepath"
)

// Chat is the inbound chat surface: one block per platform adapter. Every
// other part of bulle pushes text out; this is the one part that reads text in,
// which is why its allowlist is the security boundary of the whole feature.
type Chat struct {
	Matrix Matrix `json:"matrix" yaml:"matrix"`
}

// Matrix configures the Matrix adapter. Each credential can name a command
// instead of carrying a value (access_token_command, password_command,
// pickle_key_command), so the config file holds no secret — the same
// arrangement provider.api_key has.
//
// Allow lists the MXIDs that may start a session and Rooms limits which rooms
// are read at all. Both empty refuses every message.
type Matrix struct {
	Enabled            *bool    `json:"enabled" yaml:"enabled"`
	Homeserver         string   `json:"homeserver" yaml:"homeserver"`
	UserID             string   `json:"user_id" yaml:"user_id"`
	DeviceID           string   `json:"device_id" yaml:"device_id"`
	AccessToken        string   `json:"access_token" yaml:"access_token"`
	AccessTokenCommand string   `json:"access_token_command" yaml:"access_token_command"`
	Password           string   `json:"password" yaml:"password"`
	PasswordCommand    string   `json:"password_command" yaml:"password_command"`
	PickleKey          string   `json:"pickle_key" yaml:"pickle_key"`
	PickleKeyCommand   string   `json:"pickle_key_command" yaml:"pickle_key_command"`
	RecoveryKey        string   `json:"recovery_key" yaml:"recovery_key"`
	RecoveryKeyCommand string   `json:"recovery_key_command" yaml:"recovery_key_command"`
	Allow              []string `json:"allow" yaml:"allow"`
	Rooms              []string `json:"rooms" yaml:"rooms"`
	MaxAge             string   `json:"max_age" yaml:"max_age"`
	Workdir            string   `json:"workdir" yaml:"workdir"`
	SelfSign           *bool    `json:"self_sign" yaml:"self_sign"`
}

// ChatDir is where the chat surface keeps its own state: the crypto database
// holding the bot device's keys, and one conversation file per chat.
func ChatDir() string {
	home, err := HomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "chat")
}

// StorePath is the crypto database. It sits beside the conversation files
// rather than in a directory of its own because the two are one surface's
// state: losing either loses the thread of every conversation.
func StorePath() string {
	dir := ChatDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "matrix.db")
}

func (c *Chat) merge(over Chat) {
	c.Matrix.merge(over.Matrix)
}

func (m *Matrix) merge(over Matrix) {
	mergeBool(&m.Enabled, over.Enabled)
	mergeBool(&m.SelfSign, over.SelfSign)
	fill(&m.Homeserver, over.Homeserver)
	fill(&m.UserID, over.UserID)
	fill(&m.DeviceID, over.DeviceID)
	fill(&m.AccessToken, over.AccessToken)
	fill(&m.AccessTokenCommand, over.AccessTokenCommand)
	fill(&m.Password, over.Password)
	fill(&m.PasswordCommand, over.PasswordCommand)
	fill(&m.PickleKey, over.PickleKey)
	fill(&m.PickleKeyCommand, over.PickleKeyCommand)
	fill(&m.RecoveryKey, over.RecoveryKey)
	fill(&m.RecoveryKeyCommand, over.RecoveryKeyCommand)
	fill(&m.MaxAge, over.MaxAge)
	fill(&m.Workdir, over.Workdir)
	if len(over.Allow) > 0 {
		m.Allow = over.Allow
	}
	if len(over.Rooms) > 0 {
		m.Rooms = over.Rooms
	}
}

// fill overwrites dst when the layer above mentions the setting at all.
func fill(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}
