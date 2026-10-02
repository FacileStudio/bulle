package ide

import (
	"os"
	"path/filepath"

	"github.com/FacileStudio/bulle/internal/diff"
)

// change turns one captured edit into what an editor marks it with: the line
// span it touched in the file the call produced, and how many lines moved. It
// says no rather than guess, because a span that is not the changed one marks
// the wrong lines in somebody's editor, and an editor reading the file itself
// beats a wrong answer from here.
func (p *publisher) change(tool, path string, edit diff.EditChange) (Change, bool) {
	first, last, ok := p.span(tool, path, edit)
	if !ok {
		return Change{}, false
	}
	added, removed := diff.CountChange(edit)
	return Change{Path: path, Tool: tool, First: first, Last: last, Added: added, Removed: removed}, true
}

// span reports the changed lines in the new file. A write_file brings both
// sides with it and the two are compared directly; an edit_file brings the
// fragment it wrote, which is located in the file it landed in.
func (p *publisher) span(tool, path string, edit diff.EditChange) (int, int, bool) {
	if tool == "write_file" {
		first, last := spanOfRewrite(edit.Before, edit.After)
		return first, last, true
	}
	text, ok := p.read(path)
	if !ok {
		return 0, 0, false
	}
	return spanOfFragment(text, edit.After)
}

// read returns the file a call produced.
func (p *publisher) read(path string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(p.srv.opts.Root, filepath.FromSlash(path)))
	if err != nil {
		return "", false
	}
	return string(raw), true
}
