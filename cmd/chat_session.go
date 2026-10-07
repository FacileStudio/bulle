package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/settings"
)

func buildMatrixAdapter(m chatMatrix) (*chat.Matrix, error) {
	db, err := chat.OpenStore(settings.StorePath())
	if err != nil {
		return nil, err
	}
	adapter, err := chat.NewMatrix(chat.MatrixConfig{
		Homeserver: m.homeserver, UserID: m.userID, DeviceID: m.deviceID,
		Token: m.token, Password: m.password, PickleKey: m.pickleKey, Database: db,
		SelfSign: m.selfSign, RecoveryKey: m.recoveryKey,
	})
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if err := saveRecoveryKey(adapter.RecoveryKey()); err != nil {
		return nil, errors.Join(err, adapter.Close())
	}
	return adapter, nil
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
