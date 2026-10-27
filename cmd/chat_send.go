package cmd

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/spf13/cobra"
)

// newChatSendCmd dispatches an outbound message through the running chat daemon.
func newChatSendCmd() *cobra.Command {
	var room string
	cmd := &cobra.Command{
		Use:   "send [--room <id>] [message]",
		Short: "Send a message through the running chat daemon",
		Long:  "Send a message to a Matrix room via the running bulle chat daemon IPC socket.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChatSend(cmd.Context(), room, args, cmd.InOrStdin())
		},
	}
	cmd.Flags().StringVar(&room, "room", "", "target Matrix room id")
	return cmd
}

func runChatSend(ctx context.Context, room string, args []string, in io.Reader) error {
	msg, err := readChatSendInput(args, in)
	if err != nil {
		return err
	}
	resp, err := chat.SendIPC(ctx, chat.IPCRequest{
		Action: "send",
		Room:   room,
		Text:   msg,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error != "" {
			return errors.New(resp.Error)
		}
		return errors.New("chat daemon request failed")
	}
	return nil
}

func readChatSendInput(args []string, in io.Reader) (string, error) {
	if len(args) > 0 {
		text := strings.TrimSpace(strings.Join(args, " "))
		if text == "" {
			return "", errors.New("empty message")
		}
		return text, nil
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", errors.New("empty message")
	}
	return text, nil
}
