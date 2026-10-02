package chat

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.mau.fi/util/dbutil"

	_ "modernc.org/sqlite"
)

// OpenStore opens the sqlite database holding a bot device's state: the olm
// account, the megolm sessions, and the sync cursor that keeps a restart from
// replaying the whole backlog. The parent directory is created 0700, because
// what the file holds is the account's identity keys.
//
// The driver is modernc.org/sqlite rather than mattn/go-sqlite3 because every
// bulle release is built with CGO_ENABLED=0. The database is handed to the
// caller unopened rather than upgraded here: cryptohelper.Upgrade registers a
// child table on it at construction time, so any upgrade before that would be
// the parent's table and none of the crypto tables.
func OpenStore(path string) (*dbutil.Database, error) {
	if path == "" {
		return nil, errors.New("no chat database path")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	db, err := dbutil.NewWithDialect(storeDSN(path), "sqlite")
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	return db, nil
}

// storeDSN is the connection string modernc.org/sqlite reads. The pragmas are
// the ones a long-lived writer beside a reader needs: immediate write locks so
// a transaction cannot be promoted into a deadlock, a busy timeout so a lock
// held for a moment is waited out rather than refused, and WAL so a read never
// blocks the sync loop's write.
func storeDSN(path string) string {
	return "file:" + path +
		"?_txlock=immediate" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)"
}
