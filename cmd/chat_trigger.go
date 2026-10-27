package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/spf13/cobra"
)

// newChatTriggerCmd triggers an agent turn through the running chat daemon.
func newChatTriggerCmd() *cobra.Command {
	var room string
	cmd := &cobra.Command{
		Use:   "trigger [--room <id>] [prompt]",
		Short: "Trigger an agent turn in the running chat daemon",
		Long:  "Send a prompt to the running chat daemon and print the agent reply.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChatTrigger(cmd.Context(), room, args, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&room, "room", "", "target Matrix room id")
	return cmd
}

func runChatTrigger(ctx context.Context, room string, args []string, in io.Reader, out io.Writer) error {
	prompt, err := readChatTriggerInput(args, in)
	if err != nil {
		return err
	}
	resp, err := chat.SendIPC(ctx, chat.IPCRequest{
		Action: "trigger",
		Room:   room,
		Prompt: prompt,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if resp.Reply != "" {
		fmt.Fprintln(out, resp.Reply)
	}
	return nil
}

func readChatTriggerInput(args []string, in io.Reader) (string, error) {
	if len(args) > 0 {
		text := strings.TrimSpace(strings.Join(args, " "))
		if text == "" {
			return "", errors.New("empty prompt")
		}
		return text, nil
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", errors.New("empty prompt")
	}
	return text, nil
}
