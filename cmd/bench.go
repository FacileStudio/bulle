package cmd

import (
	"errors"
	"strings"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/spf13/cobra"
)

type benchOptions struct {
	runs int
	json bool
}

func newBenchCmd() *cobra.Command {
	var opts benchOptions
	cmd := &cobra.Command{
		Use:   "bench [flags] [prompt]",
		Short: "Benchmark repeated headless runs for timing, tokens, and cost",
		Long: "Run a prompt through the headless agent sequentially to benchmark\n" +
			"duration, input/output/cache tokens, cost, tool calls, and context size.",
		RunE: func(_ *cobra.Command, args []string) error {
			return runBenchCommand(&opts, args)
		},
	}
	cmd.Flags().IntVarP(&opts.runs, "runs", "n", 1, "Number of sequential runs")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Print results as a single JSON document")
	return cmd
}

func runBenchCommand(opts *benchOptions, args []string) error {
	prompt, err := resolveBenchPrompt(args)
	if err != nil {
		return err
	}
	return agent.RunBench(prompt, opts.runs, opts.json)
}

func resolveBenchPrompt(args []string) (string, error) {
	prompt := strings.Join(args, " ")
	if prompt != "" {
		return prompt, nil
	}
	piped, err := agent.StdinPrompt()
	if err != nil || piped == "" {
		return "", errors.New("no prompt provided: pass a prompt argument or pipe via stdin")
	}
	return piped, nil
}
