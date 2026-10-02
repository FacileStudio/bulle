package cmd

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

// chatHistoryLimit caps how much of a conversation is replayed into the prompt,
// because a chat that has run for months would otherwise grow the prompt
// forever. Nothing is ever deleted from the file: the whole record stays, and
// only the last turns are sent to the model.
const chatHistoryLimit = 50

const (
	chatWhoUser      = "user"
	chatWhoAssistant = "assistant"
)

// chatTurn is one line of a conversation file: who spoke and what they said.
// Text is stored rather than a nacelle.Message because that message's Part is
// an interface, which JSON cannot round trip; the messages are rebuilt with
// nacelle.UserText and nacelle.AssistantText on load.
type chatTurn struct {
	Who  string `json:"who"`
	Text string `json:"text"`
}

// chatSessionPath is the file holding one conversation. The name is a hash of
// the session key because a Matrix room id carries '!' and ':', which have no
// business in a path; the key itself is written on the file's first line so a
// human can still tell which chat a file belongs to.
func chatSessionPath(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(settings.ChatDir(), "sessions", hex.EncodeToString(sum[:16])+".jsonl")
}

// loadChatTurns reads a conversation and returns its last chatHistoryLimit
// turns, oldest first. A file that does not exist yet is an empty conversation,
// and a line that will not parse — a crash mid-append leaves one — is skipped
// rather than costing the thread every turn around it.
func loadChatTurns(key string) ([]chatTurn, error) {
	file, err := os.Open(chatSessionPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var turns []chatTurn
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var turn chatTurn
		if json.Unmarshal([]byte(line), &turn) != nil {
			continue
		}
		turns = append(turns, turn)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	if len(turns) > chatHistoryLimit {
		turns = turns[len(turns)-chatHistoryLimit:]
	}
	return turns, nil
}

// appendChatTurn adds one turn to a conversation, creating the file 0600 and
// its directory 0700 first. The header line is written only when the file is
// empty, so an append never rewrites what is already there.
func appendChatTurn(key, who, text string) error {
	path := chatSessionPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if info, err := file.Stat(); err == nil && info.Size() == 0 {
		if _, err := fmt.Fprintf(file, "# bulle chat session %s\n", key); err != nil {
			return err
		}
	}
	line, err := json.Marshal(chatTurn{Who: who, Text: text})
	if err != nil {
		return err
	}
	_, err = file.Write(append(line, '\n'))
	return err
}

// chatMessages rebuilds the conversation the model sees, ending with the new
// question. Assistant text is reconstructed with nacelle.AssistantText and the
// human's with nacelle.UserText, the constructors that produce the text-part
// messages this surface persists.
func chatMessages(turns []chatTurn, question string) []nacelle.Message {
	conv := make([]nacelle.Message, 0, len(turns)+1)
	for _, turn := range turns {
		if turn.Who == chatWhoUser {
			conv = append(conv, nacelle.UserText(turn.Text))
			continue
		}
		conv = append(conv, nacelle.AssistantText(turn.Text))
	}
	return append(conv, nacelle.UserText(question))
}

// expandTilde replaces a leading ~ with the user's home directory, the
// expansion the config documents for a workdir.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// saveRecoveryKey writes the cross-signing recovery key the first time a bot
// generates its identity, and does nothing on every run after. The key is what
// recovers the signing keys onto a fresh database, since the only other copy
// lives in the account's server-side SSSS, so it is written 0600 beside the
// pickle key and never overwritten: a later key would be a second identity.
func saveRecoveryKey(key string) error {
	if key == "" {
		return nil
	}
	path := filepath.Join(settings.ChatDir(), "recovery.key")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(settings.ChatDir(), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "bulle chat: cross-signing recovery key written to %s; keep it, it is the only copy\n", path)
	return nil
}

// loadRecoveryKey reads the stored cross-signing recovery key if one was
// previously written to ~/.bulle/chat/recovery.key.
func loadRecoveryKey() string {
	path := filepath.Join(settings.ChatDir(), "recovery.key")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// loadOrCreatePickleKey returns the key that encrypts the bot device's stored
// keys. A configured value or command wins; with neither, a fresh key is
// generated once and kept beside the database 0600, because a key that changed
// on every start would make bulle a new device each time and flood the
// homeserver. Shipping one constant instead would make every bulle install share
// a key, which is the same as having none.
func loadOrCreatePickleKey(m settings.Matrix) ([]byte, error) {
	if m.PickleKey != "" {
		return []byte(m.PickleKey), nil
	}
	if m.PickleKeyCommand != "" {
		key, err := settings.KeyFromCommand(m.PickleKeyCommand)
		if err != nil {
			return nil, &settings.ParseError{Path: "chat.matrix.pickle_key_command", Err: err}
		}
		return []byte(key), nil
	}
	path := filepath.Join(settings.ChatDir(), "pickle.key")
	if stored, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(stored)) > 0 {
		return bytes.TrimSpace(stored), nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	key := []byte(hex.EncodeToString(raw))
	if err := os.MkdirAll(settings.ChatDir(), 0o700); err != nil {
		return nil, err
	}
	return key, os.WriteFile(path, key, 0o600)
}
