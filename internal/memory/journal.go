package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const journalTemplate = `---
id: %[1]s
aliases:
  - Journal %[1]s
  - Daily %[1]s
tags: [journal, daily]
date: %[1]s
---

# %[1]s

---
## Rêve


---
## Journal

`

// JournalPath is the daily journal file for a day inside dir.
func JournalPath(dir string, day time.Time) string {
	return filepath.Join(dir, day.Format("2006-01-02")+".md")
}

// EnsureJournal creates the daily journal from the vault template when it does
// not exist yet, so an entry appended under "## Journal" lands in a file that
// keeps its frontmatter and sections. An existing file is never touched.
func EnsureJournal(dir string, day time.Time) (string, error) {
	path := JournalPath(dir, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	_, werr := fmt.Fprintf(file, journalTemplate, day.Format("2006-01-02"))
	return path, errors.Join(werr, file.Close())
}
