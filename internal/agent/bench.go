package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

// The two outcomes a bench run can report.
const (
	benchOK     = "ok"
	benchFailed = "failed"
)

// printBenchUsage writes the bench subcommand's usage to stdout; help exits 0.
func printBenchUsage() error {
	fmt.Println(`usage: bulle bench [flags] <prompt>

  Run the prompt through the headless path one or more times, sequentially,
  and print what each run measured: duration, input, output and cache
  tokens, cost, tool calls, and the final context size.

  -n runs          how many times to run the prompt (default 1)
  --json           print one JSON document instead of the text table

  The prompt comes from the arguments, or from stdin when piped. Settings
  flags (-model, -backend, ...) go ahead of the word bench, where the
  settings parser reads them.`)
	return nil
}

// benchFlags is what the bench subcommand parsed from its own arguments.
type benchFlags struct {
	prompt string
	runs   int
	json   bool
}

// checkBenchFlag handles the `bench` subcommand, which times repeated headless
// runs of one prompt. It returns true when it recognised a bench command, and
// the caller should then treat its error as the process's result.
func checkBenchFlag() (bool, error) {
	_, sub, ok := findCommand("bench")
	if !ok {
		return false, nil
	}
	if len(sub) > 0 {
		switch sub[0] {
		case "help", "-h", "--help":
			return true, printBenchUsage()
		}
	}
	flags, err := parseBenchFlags(sub)
	if err != nil {
		return true, err
	}
	if flags.prompt == "" {
		piped, perr := stdinPrompt()
		if perr != nil || piped == "" {
			return true, usagef("usage: bulle bench [-n runs] [--json] <prompt>")
		}
		flags.prompt = piped
	}
	return true, runBench(flags)
}

// parseBenchFlags reads the bench subcommand's own arguments: -n runs and
// --json, with everything left over joined into the prompt.
func parseBenchFlags(sub []string) (benchFlags, error) {
	flags := benchFlags{runs: 1}
	rest, err := benchScanArgs(sub, &flags)
	if err != nil {
		return benchFlags{}, err
	}
	flags.prompt = strings.Join(rest, " ")
	return flags, nil
}

// benchScanArgs classifies each argument: the --json switch, a run-count flag
// in either form, or prompt text.
func benchScanArgs(sub []string, flags *benchFlags) ([]string, error) {
	var rest []string
	for i := 0; i < len(sub); i++ {
		value, used, isRuns := benchRunsArg(sub[i], sub, i)
		switch {
		case sub[i] == "-json" || sub[i] == "--json":
			flags.json = true
		case isRuns:
			i += used
			if err := benchRunCount(value, flags); err != nil {
				return nil, err
			}
		default:
			rest = append(rest, sub[i])
		}
	}
	return rest, nil
}

// benchRunsArg reports whether the argument is a run-count flag, with the
// value it names and how many arguments it consumed: the space form reads the
// next argument, the equals form carries its own.
func benchRunsArg(arg string, sub []string, i int) (value string, used int, ok bool) {
	if arg == "-n" || arg == "--n" {
		if i+1 < len(sub) {
			return sub[i+1], 1, true
		}
		return "", 0, true
	}
	for _, prefix := range []string{"-n=", "--n="} {
		if v, found := strings.CutPrefix(arg, prefix); found {
			return v, 0, true
		}
	}
	return "", 0, false
}

// benchRunCount parses one -n value into flags.runs, refusing anything but a
// positive count.
func benchRunCount(value string, flags *benchFlags) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return usagef("invalid -n %q: want a positive run count", value)
	}
	flags.runs = n
	return nil
}

// runBench times the requested runs one after another and prints the report,
// text or JSON. The settings resolve once, here, since the flag parser cannot
// run twice in one process.
func runBench(flags benchFlags) error {
	config, err := loadCronConfig()
	if err != nil {
		return err
	}
	results := make([]benchRun, 0, flags.runs)
	for i := 1; i <= flags.runs; i++ {
		if flags.runs > 1 {
			fmt.Fprintf(os.Stderr, "bench: run %d/%d\n", i, flags.runs)
		}
		results = append(results, runBenchOnce(i, flags.prompt, config))
	}
	if flags.json {
		data, err := benchJSON(results, config.Model)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(benchTextTable(results))
	return nil
}

// benchRun is one timed run of the bench loop: the run's number, how long it
// took, what it measured, and the error that failed it, if any.
type benchRun struct {
	Run      int
	Duration time.Duration
	Stats    runStats
	Err      error
}

// runBenchOnce streams the prompt once through a freshly built agent,
// discarding the text, and returns what the run measured. A failure ends
// that run only; the loop keeps going.
func runBenchOnce(number int, prompt string, config settings.Config) benchRun {
	run := benchRun{Run: number}
	started := time.Now()
	_, stats, err := runHeadlessConfigToContext(context.Background(), io.Discard, prompt, config, nil)
	run.Duration = time.Since(started)
	run.Stats = stats
	run.Err = err
	return run
}
