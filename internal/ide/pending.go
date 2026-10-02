package ide

import (
	"fmt"

	"github.com/FacileStudio/bulle/internal/diff"
)

// begin allocates the id a call's start and finish share, and remembers it
// under the tool name that pairs them: a hook is told a tool name and not a
// call, so the name is the only thing there is to pair on.
func (p *publisher) begin(tool string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.nextID(tool)
	p.open[tool] = id
	return id
}

// finish returns the id the matching start used. A finish with no start
// behind it still gets an id of its own, so no client is handed a done for a
// call it was never told about.
func (p *publisher) finish(tool string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	id, open := p.open[tool]
	if !open {
		return p.nextID(tool)
	}
	delete(p.open, tool)
	return id
}

// remember records the change a call is about to make.
func (p *publisher) remember(tool string, change diff.EditChange) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending[tool] = change
}

// take returns the change a call made, and forgets it.
func (p *publisher) take(tool string) (diff.EditChange, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	change, ok := p.pending[tool]
	delete(p.pending, tool)
	return change, ok
}

// nextID numbers one call within its tool. The caller holds mu.
func (p *publisher) nextID(tool string) string {
	p.seq++
	return fmt.Sprintf("%s-%d", tool, p.seq)
}
