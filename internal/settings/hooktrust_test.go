package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestIsTrustedRefusesChangedContentUntilSavedAgain pins the trust contract
// the hooks and jobs gates share: a saved approval holds for exactly the
// bytes it covered, and one byte changed re-arms it until someone saves
// again.
func TestIsTrustedRefusesChangedContentUntilSavedAgain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "hooks.yml")
	raw := []byte("hooks:\n  - on: before_tool_call\n    run: echo hi\n")

	if trusted, err := IsTrusted(path, raw); err != nil || trusted {
		t.Fatalf("untrusted content: trusted=%v err=%v", trusted, err)
	}
	if err := Save(path, raw); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if trusted, _ := IsTrusted(path, raw); !trusted {
		t.Fatal("an approval just saved was not remembered")
	}

	changed := append(raw, '\n')
	if trusted, _ := IsTrusted(path, changed); trusted {
		t.Fatal("one changed byte stayed trusted")
	}
	if err := Save(path, changed); err != nil {
		t.Fatalf("Save after edit: %v", err)
	}
	if trusted, _ := IsTrusted(path, changed); !trusted {
		t.Fatal("re-saving the changed content did not trust it")
	}
}

// TestHooksJSONApprovalsSurviveTheSharedStore keeps the migration honest: an
// approval recorded in the old hooks.json is honored by the shared store.
func TestHooksJSONApprovalsSurviveTheSharedStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "project", HooksFile)
	raw := []byte("hooks:\n  - on: before_tool_call\n    run: echo hi\n")
	writeOldHooksTrust(t, home, path, raw)

	if trusted, _ := IsTrusted(path, raw); !trusted {
		t.Fatal("an approval recorded in hooks.json was not honored")
	}
}

// TestTheFirstSaveFoldsHooksJSONForward checks that the first Save migrates
// the old store: trust.json appears, and the approval it inherited still
// holds once the fallback no longer matters.
func TestTheFirstSaveFoldsHooksJSONForward(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "project", HooksFile)
	raw := []byte("hooks:\n  - on: before_tool_call\n    run: echo hi\n")
	writeOldHooksTrust(t, home, path, raw)

	other := filepath.Join(home, "elsewhere.yml")
	if err := Save(other, []byte("x")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".bulle", TrustFile)); err != nil {
		t.Fatalf("Save did not write %s: %v", TrustFile, err)
	}
	if trusted, _ := IsTrusted(path, raw); !trusted {
		t.Fatal("the hooks.json approval fell away once trust.json existed")
	}
	if trusted, _ := IsTrusted(other, []byte("x")); !trusted {
		t.Fatal("the new approval was not remembered")
	}
}

// writeOldHooksTrust records one approval the way the pre-migration store
// did: in hooks.json, with trust.json not yet in the picture.
func writeOldHooksTrust(t *testing.T, home, path string, raw []byte) {
	t.Helper()
	store := map[string]trustRecord{path: {Hash: contentHash(raw), TrustedAt: "2026-01-01T00:00:00Z"}}
	dir := filepath.Join(home, ".bulle")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, HookTrustFile), old, 0o644); err != nil {
		t.Fatal(err)
	}
}
