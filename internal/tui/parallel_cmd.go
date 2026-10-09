package tui

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// handleParallelCommand parses a `/parallel task1, task2, task3` line and
// launches the fan-out. It does not go through the parent model: the work runs
// in detached nested agents cloned from the main agent's own Config, and the
// main thread stays free — the point of /parallel is that you keep chatting
// while the parallel_agents grind. The model's own `parallel_agents` tool is the
// same detached deal: it returns a stub immediately and the fan-out streams
// back through the Results hook, so neither path pins the main thread to the
// fan-out's duration.
//
// Tasks are split on `,`, trimmed, and empty entries dropped. A single task
// still routes through the parallel machinery — its input is a task list, so a
// one-item list is the smallest call.
func (m *Model) handleParallelCommand(args string) tea.Cmd {
	if args == "cancel" || strings.HasPrefix(args, "cancel ") {
		return m.cancelParallelCommand(strings.TrimPrefix(args, "cancel"))
	}
	tasks, titles := splitParallelTasksWithTitles(args)
	if len(tasks) == 0 {
		m.say(fromClient, "usage: /parallel task1, task2, task3")
		return nil
	}
	if m.delegate.Backend == nil {
		m.say(fromFailure, "no agent is configured to run parallel agents")
		return nil
	}
	return m.launchDetached(tasks, titles)
}

// cancelParallelCommand stops live detached fan-outs: `/parallel cancel` every
// one, `/parallel cancel psa-3` just that batch. The SDK's CancelParallel kills
// the fan-out's context — each still-running task then ends on the Results
// stream as a cancelled error — and recordDetached marks the rows failed so the
// status lines stop spinning. Tasks that finished before the cancel keep their
// results.
func (m *Model) cancelParallelCommand(args string) tea.Cmd {
	args = strings.TrimSpace(args)
	live := m.liveParallelBatches(args)
	if len(live) == 0 {
		m.say(fromClient, "no live parallel fan-out to cancel")
		return nil
	}
	for _, batch := range live {
		if cancel, ok := m.parallelCancels[batch]; ok {
			cancel()
			delete(m.parallelCancels, batch)
		}
		m.recordDetached(detachedResult{batch: batch, idx: -1, err: "cancelled"})
	}
	m.say(fromClient, "cancelled "+countedNoun(len(live), "fan-out")+" ("+strings.Join(live, ", ")+")")
	return nil
}

func (m *Model) liveParallelBatches(only string) []string {
	live := make([]string, 0, len(m.parallelTasks))
	for batch, tasks := range m.parallelTasks {
		if only != "" && batch != only {
			continue
		}
		for _, pt := range tasks {
			if pt.Active {
				live = append(live, batch)
				break
			}
		}
	}
	sort.Strings(live)
	return live
}

// splitParallelTasks turns `/parallel a, b , c` into ["a", "b", "c"].
func splitParallelTasks(args string) []string {
	tasks, _ := splitParallelTasksWithTitles(args)
	return tasks
}

func splitParallelTasksWithTitles(args string) ([]string, []string) {
	parts := strings.Split(args, ",")
	tasks := make([]string, 0, len(parts))
	titles := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			title, prompt := parseInlineTaskTitle(t)
			tasks = append(tasks, prompt)
			titles = append(titles, title)
		}
	}
	return tasks, titles
}

// delegateApprove is the approval policy the detached parallel_agents answer to. It
// is the same rule agent.delegateApprovals applies to the model-callable tool:
// a delegate inherits the parent's policy, and a session with approvals off
// hands the delegate an allow-all rather than the SDK's deny-all default.
// The rule lives here rather than in agent because agent imports tui, so tui
// cannot import it back — this is the mirror that keeps the two linked.
func delegateApprove(cfg nacelle.Config) nacelle.Approve {
	if cfg.Approve != nil {
		return cfg.Approve
	}
	return func(context.Context, string, json.RawMessage) bool { return true }
}
