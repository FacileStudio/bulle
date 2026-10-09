package tui

import (
	"context"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/FacileStudio/nacelle"
)

// This file is the detached /parallel fan-out: the launcher that runs nested
// agents off the main run, and the result handlers that fold their outcomes
// back into the update loop. It is separate from parallel_cmd.go (the parser)
// to stay under filter's per-file function cap; parallel_result.go owns the
// model-callable tool's result path.

// detachedResult is one subagent's outcome, or a launch failure, on its way
// back to the update loop from the fan-out goroutine. idx is the task's index
// in the batch; a negative idx means the whole batch failed to start and marks
// every still-running task.
type detachedResult struct {
	batch  string
	idx    int
	result string
	err    string
	usage  nacelle.Usage
}

// detached is the channel the fan-out goroutine posts results to, drained by
// watchDetached on the update loop — the one thread that may touch the task
// rows and the session total.
var detached = make(chan detachedResult, 64)

func watchDetached() tea.Cmd {
	return func() tea.Msg {
		return <-detached
	}
}

// launchDetached starts one fan-out without touching the main run. It registers
// the batch's row state, says in the main thread that the agents were started,
// and hands the actual delegation to a background goroutine; busy stays false,
// so the prompt stays live. The tick the register returns is the only thing
// that wakes the loop while nothing else is happening, so the elapsed clocks
// under the prompt move for a fan-out launched from an idle prompt.
func (m *Model) launchDetached(tasks []string, titles ...[]string) tea.Cmd {
	id := m.nextDetachID()
	tick := m.registerParallel(id, tasks, titles...)
	ctx, cancel := context.WithCancel(context.Background())
	m.parallelCancels[id] = cancel
	cbs := nacelle.ParallelSubAgentCallbacks{
		Tool: func(b string, i int, t string) { subagentUpdates <- subagentUpdate{batch: id, idx: i, tool: t} },
		LiveUsage: func(b string, i int, u nacelle.Usage) {
			subagentUpdates <- subagentUpdate{batch: id, idx: i, usage: u, spend: true}
		},
		ToolDone: ReportSubagentDone,
	}
	go func() {
		results, err := nacelle.DelegateParallel(ctx, m.delegate, tasks, nacelle.ParallelSubAgentOptions{
			Approve:        delegateApprove(m.delegate),
			Callbacks:      cbs,
			MaxConcurrency: max(m.maxConcurrency, 16),
		})
		if err != nil {
			detached <- detachedResult{batch: id, idx: -1, err: err.Error()}
			return
		}
		for {
			next, open := <-results
			if !open {
				break
			}
			detached <- detachedResult{batch: id, idx: next.Index, result: next.Result, err: next.Err, usage: next.Usage}
		}
		delete(m.parallelCancels, id)
	}()
	return tick
}

// registerParallel seeds the row state for a batch and announces it in the main
// thread. Both the /parallel command and a model's non-blocking tool call end up
// here; the batch key is theirs to choose. The fan-out's streamed results route
// back by that same key. It returns the spinner tick so a fan-out launched from
// an idle prompt wakes the loop; spun keeps the tick alive while the task rows
// are still live.
func (m *Model) registerParallel(batch string, tasks []string, titles ...[]string) tea.Cmd {
	if m.parallelTasks == nil {
		m.parallelTasks = make(map[string][]parallelTaskInfo)
		m.parallelCancels = make(map[string]context.CancelFunc)
	}
	var provided []string
	if len(titles) > 0 {
		provided = titles[0]
	}
	list := make([]parallelTaskInfo, len(tasks))
	var needTitles bool
	for i, t := range tasks {
		var title string
		if i < len(provided) {
			title = provided[i]
		}
		list[i] = parallelTaskInfo{Task: t, Title: title, Began: time.Now(), Active: true}
		if title == "" {
			needTitles = true
		}
	}
	m.parallelTasks[batch] = list
	if needTitles {
		m.titleParallelTasks(batch, tasks)
	}
	m.announceStart(len(tasks))
	m.layout(m.windowHeight)
	return m.spin.Tick
}

// announceStart, the fan-out confirmation, lives in parallel_wait.go.

// nextDetachID hands out a batch key for a detached fan-out. The "detach"
// prefix keeps it apart from the model-tool path's nacelle batch keys, which
// are the other kind of key the parallelTasks map holds.
func (m *Model) nextDetachID() string {
	m.detachedSeq++
	return "detach" + strconv.Itoa(m.detachedSeq)
}

// recordDetached applies one detached subagent result and re-arms the watch.
// A task's own spend joins the session total so the footer does not lie about
// work that ran outside any parent run. When the last task lands and the main
// run is idle, it wakes the main agent to review the completed fan-out.
func (m *Model) recordDetached(r detachedResult) tea.Cmd {
	if tasks, ok := m.parallelTasks[r.batch]; ok {
		m.applyDetached(tasks, r)
		m.layout(m.windowHeight)
	}
	if m.parallelReview() {
		m.grind.used = 0
		return tea.Batch(watchDetached(), m.send(m.reviewText()))
	}
	return watchDetached()
}

// applyDetached routes one result to its task, or — negative index, the
// launch-failed marker — fails every still-running task in the batch.
func (m *Model) applyDetached(tasks []parallelTaskInfo, r detachedResult) {
	if r.idx >= 0 && r.idx < len(tasks) {
		m.finishDetached(&tasks[r.idx], r)
		return
	}
	for i := range tasks {
		m.failDetached(tasks, i, r.err)
	}
}

// finishDetached records a task's outcome and spend, and marks it done. The
// task's own spend joins the session total here; for a detached fan-out some of
// it may already have been folded live, so only the residual over what was is
// added — otherwise live updates and the authoritative figure double-count.
// Like failDetached it only touches running tasks: a cancelled fan-out's
// recordDetached call has already marked them, and a late or duplicate result
// must not overwrite a finished task.
func (m *Model) finishDetached(pt *parallelTaskInfo, r detachedResult) {
	if !pt.Active {
		return
	}
	if r.err != "" {
		pt.Err = r.err
	} else {
		pt.Result = r.result
	}
	pt.Usage = r.usage
	pt.End = time.Now()
	pt.Active = false
	pt.Tool = ""
	pt.ToolOut = ""
	if r.usage.Total() > 0 {
		residual := nacelle.Usage{
			InputTokens:         max(r.usage.InputTokens-pt.Ledgered.InputTokens, 0),
			OutputTokens:        max(r.usage.OutputTokens-pt.Ledgered.OutputTokens, 0),
			CacheReadTokens:     max(r.usage.CacheReadTokens-pt.Ledgered.CacheReadTokens, 0),
			CacheCreationTokens: max(r.usage.CacheCreationTokens-pt.Ledgered.CacheCreationTokens, 0),
			Cost:                max(r.usage.Cost-pt.Ledgered.Cost, 0),
		}
		m.spent = m.spent.Add(residual)
	}
}

// failDetached marks one task failed; it only touches running tasks, so a
// launch error after some results arrived never clobbers what already landed.
func (m *Model) failDetached(tasks []parallelTaskInfo, i int, err string) {
	if !tasks[i].Active {
		return
	}
	tasks[i].Err = err
	tasks[i].End = time.Now()
	tasks[i].Active = false
}
