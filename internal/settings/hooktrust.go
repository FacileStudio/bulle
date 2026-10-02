package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// TrustFile is the shared approval store: one record per absolute path,
// naming the content hash a human trusted. Hook files and job files both
// land here.
const TrustFile = "trust.json"

// HookTrustFile is the hooks-only store the shared one replaced. It is still
// read, under TrustFile, so approvals recorded before the shared store
// existed keep working.
const HookTrustFile = "hooks.json"

// trustRecord is one trusted file: the hash that was approved, and when,
// for the human reading the store by hand.
type trustRecord struct {
	Hash      string `json:"hash"`
	TrustedAt string `json:"trustedAt"`
}

// trustDir is where the trust store lives — the first thing this package
// puts under ~/.bulle/, which stays otherwise empty until something needs it.
func trustDir() (string, error) {
	return HomeDir()
}

// loadTrust reads every saved approval: TrustFile, with HookTrustFile read
// under it for paths the older store alone still holds. Neither file yet is
// the ordinary first-run case, not an error.
func loadTrust() (map[string]trustRecord, error) {
	dir, err := trustDir()
	if err != nil {
		return nil, err
	}
	store, err := readTrust(filepath.Join(dir, TrustFile))
	if err != nil {
		return nil, err
	}
	old, err := readTrust(filepath.Join(dir, HookTrustFile))
	if err != nil {
		return nil, err
	}
	for path, record := range old {
		if _, kept := store[path]; !kept {
			store[path] = record
		}
	}
	return store, nil
}

// readTrust decodes one store file; a file that is not there yet is empty.
func readTrust(path string) (map[string]trustRecord, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]trustRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	store := map[string]trustRecord{}
	if err := json.Unmarshal(raw, &store); err != nil {
		return nil, err
	}
	return store, nil
}

// saveTrust records one approval, creating ~/.bulle/ if the store has not
// needed it before now.
func saveTrust(store map[string]trustRecord, path, hash string) error {
	dir, err := trustDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	store[path] = trustRecord{Hash: hash, TrustedAt: time.Now().UTC().Format(time.RFC3339)}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, TrustFile), raw, 0o644)
}

// contentHash is what the trust store keys on: one byte changed is a
// different file.
func contentHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// IsTrusted reports whether this exact content at this exact path was
// approved before. One byte changed is a different file.
func IsTrusted(path string, raw []byte) (bool, error) {
	store, err := loadTrust()
	if err != nil {
		return false, err
	}
	record, seen := store[path]
	return seen && record.Hash == contentHash(raw), nil
}

// Save records an approval for this exact content at this exact path.
func Save(path string, raw []byte) error {
	store, err := loadTrust()
	if err != nil {
		return err
	}
	return saveTrust(store, path, contentHash(raw))
}
