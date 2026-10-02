package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// storePath builds a path shaped like ~/.bulle/chat/crypto.db whose parent does
// not exist yet, so OpenStore has to create it and its mode is part of what
// the tests check.
func storePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "chat", "crypto.db")
}

// TestOpenStoreCreatesDirectory pins the permission on the directory the store
// lives in. The file holds a bot device's identity keys, so the directory must
// be 0700 rather than whatever the process umask happens to be.
func TestOpenStoreCreatesDirectory(t *testing.T) {
	path := storePath(t)
	db, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore(%q): %v", path, err)
	}
	defer db.Close()

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("parent directory mode = %o, want 700", perm)
	}
}

// TestOpenStoreSpeaksSQLite proves the returned database really is sqlite on
// this machine. A driver that is not linked in fails at the first connection,
// which is what catches modernc.org/sqlite silently missing at CGO_ENABLED=0.
func TestOpenStoreSpeaksSQLite(t *testing.T) {
	db, err := OpenStore(storePath(t))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.RawDB.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	var one int
	if err := db.RawDB.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 returned %d, want 1", one)
	}
}

// TestOpenStoreReopens is the restart case: the daemon opens the same file
// after a crash or a redeploy, and the second open must not fail on a database
// that already exists.
func TestOpenStoreReopens(t *testing.T) {
	path := storePath(t)
	first, err := OpenStore(path)
	if err != nil {
		t.Fatalf("first OpenStore: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("closing first store: %v", err)
	}

	second, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopening %q: %v", path, err)
	}
	defer second.Close()
	if err := second.RawDB.PingContext(context.Background()); err != nil {
		t.Fatalf("ping after reopen: %v", err)
	}
}

// TestOpenStoreEmptyPath refuses an empty path with an error rather than a
// panic, because the path comes from config and a missing value is the likely
// first thing a new install hits.
func TestOpenStoreEmptyPath(t *testing.T) {
	db, err := OpenStore("")
	if err == nil {
		t.Fatal("OpenStore(\"\") returned no error")
	}
	if db != nil {
		t.Fatal("OpenStore(\"\") returned a database alongside the error")
	}
	if !strings.Contains(err.Error(), "no chat database path") {
		t.Fatalf("error text changed: %v", err)
	}
}

// TestStoreDSN pins every pragma the DSN carries. The store is a long-lived
// writer beside a reader: dropping _txlock risks a transaction promoted into a
// deadlock, dropping busy_timeout refuses a lock held for a moment, and
// dropping WAL lets a read starve the sync loop's write. foreign_keys guards
// the crypto tables' relations.
func TestStoreDSN(t *testing.T) {
	dsn := storeDSN("/tmp/crypto.db")
	for _, want := range []string{
		"file:/tmp/crypto.db",
		"_txlock=immediate",
		"_pragma=foreign_keys(1)",
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN %q is missing %q", dsn, want)
		}
	}
}
