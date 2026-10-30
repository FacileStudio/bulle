package assistant

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const maxSenderMap = 2000

func readJSONFile(path string, v any) {
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, v); err != nil {
			return
		}
	}
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// RecordMailSenders remembers which address sent each mail key, so a 👎 on a
// digest line can find the sender to drop.
func RecordMailSenders(stateDir string, senders map[string]string) error {
	if stateDir == "" || len(senders) == 0 {
		return nil
	}
	return withLock(filepath.Join(stateDir, "mail-senders.lock"), true, func() error {
		path := filepath.Join(stateDir, "mail-senders.json")
		m := map[string]string{}
		readJSONFile(path, &m)
		maps.Copy(m, senders)
		for k := range m {
			if len(m) <= maxSenderMap {
				break
			}
			delete(m, k)
		}
		return writeJSONFile(path, m)
	})
}

// LoadDropSenders returns the senders added by 👎 reactions.
func LoadDropSenders(stateDir string) []string {
	var list []string
	readJSONFile(filepath.Join(stateDir, "drop-senders.json"), &list)
	return list
}

// DropSenderOf appends the sender address recorded for a mail key to
// drop-senders.json. An unknown key does nothing.
func DropSenderOf(stateDir, key string) error {
	m := map[string]string{}
	readJSONFile(filepath.Join(stateDir, "mail-senders.json"), &m)
	addr := strings.ToLower(strings.TrimSpace(m[key]))
	if addr == "" {
		return nil
	}
	return withLock(filepath.Join(stateDir, "drop-senders.lock"), true, func() error {
		path := filepath.Join(stateDir, "drop-senders.json")
		list := LoadDropSenders(stateDir)
		if slices.Contains(list, addr) {
			return nil
		}
		return writeJSONFile(path, append(list, addr))
	})
}
