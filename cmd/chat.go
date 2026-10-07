package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

// chatMatrix is a configuration past its refusals, with credentials resolved.
type chatMatrix struct {
	homeserver  string
	userID      string
	deviceID    string
	token       string
	password    string
	pickleKey   []byte
	recoveryKey string
	allow       []string
	rooms       []string
	maxAge      time.Duration
	workdir     string
	selfSign    bool
}

// newChatCmd builds the chat surface: the daemon itself, plus the commands that
// list its adapters and put it under a systemd user unit.
func newChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat [command]",
		Short: "Run the inbound chat daemon and manage it",
		Long: `Hold the configured chat adapters open and answer inbound messages.

With no subcommand, bulle chat runs the daemon: it connects each adapter,
checks every message against the allowlist, runs one headless session per
conversation, and answers back in the room it came from. Every message is
untrusted text, so the allowlist and the tool policy stay in bulle.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runChatDaemon()
		},
	}
	cmd.AddCommand(newChatChannelsCmd())
	cmd.AddCommand(newChatInstallCmd())
	cmd.AddCommand(newChatUninstallCmd())
	cmd.AddCommand(newChatVerifyCmd())
	return cmd
}

// runChatDaemon holds the adapters open until the process is interrupted. The
// allowlist and the credentials are resolved before the first adapter is built,
// because a daemon that starts and then discovers it may answer nobody has
// already connected to the room.
func runChatDaemon() error {
	config, err := agent.ChatConfig()
	if err != nil {
		return err
	}
	m, err := resolveChatMatrix(config)
	if err != nil {
		return err
	}
	if m.workdir != "" {
		config.Root = expandTilde(m.workdir)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	adapter, err := buildMatrixAdapter(m)
	if err != nil {
		return err
	}
	router := chat.Router{Allow: m.allow, Rooms: m.rooms, MaxAge: m.maxAge}
	fmt.Fprintf(os.Stderr, "bulle chat: %s listening as %s on %s\n", adapter.Name(), m.userID, m.homeserver)
	rooms := newRoomRegistry(config)
	defer func() { _ = rooms.Close() }()
	return errors.Join(chat.Run(ctx, adapter, chatResponder(rooms), router), adapter.Close())
}

// chatResponder routes incoming messages through the room's session.
func chatResponder(rooms *roomRegistry) chat.Responder {
	return func(ctx context.Context, m chat.Message) (string, error) {
		room, err := rooms.get(m.Key())
		if err != nil {
			return "", err
		}
		return room.answer(ctx, m.Text)
	}
}

// validateChatMatrix refuses the configurations that must never reach the
// agent. The allowlist is checked here, before an adapter is built, because it
// is the only wall between an untrusted room and a shell: an adapter started
// without one has already accepted the messages it would need to refuse.
func validateChatMatrix(m settings.Matrix) error {
	if !settings.DerefBool(m.Enabled) {
		return errors.New("chat.matrix is not enabled: set `enabled: true` under chat in ~/.bulle.yml")
	}
	if m.Homeserver == "" {
		return errors.New("chat.matrix.homeserver is empty: set the homeserver base URL")
	}
	if m.UserID == "" {
		return errors.New("chat.matrix.user_id is empty: set the bot's MXID")
	}
	if len(m.Allow) == 0 {
		return errors.New("chat.matrix.allow is empty: refusing every message, because the allowlist is the security boundary")
	}
	return nil
}

// resolveCredentials fills the bot's token and password. A command runs only to
// fill what the config left empty, the same source-not-override rule
// provider.api_key_command follows, and a command that fails is a load error
// naming the field rather than an empty credential that reads like a typo.
func resolveCredentials(m settings.Matrix) (string, string, error) {
	token := m.AccessToken
	var err error
	if token == "" && m.AccessTokenCommand != "" {
		if token, err = settings.KeyFromCommand(m.AccessTokenCommand); err != nil {
			return "", "", &settings.ParseError{Path: "chat.matrix.access_token_command", Err: err}
		}
	}
	password := m.Password
	if password == "" && m.PasswordCommand != "" {
		if password, err = settings.KeyFromCommand(m.PasswordCommand); err != nil {
			return "", "", &settings.ParseError{Path: "chat.matrix.password_command", Err: err}
		}
	}
	if token == "" && password == "" {
		return "", "", errors.New("chat.matrix has no credential: set access_token, access_token_command, password, or password_command")
	}
	return token, password, nil
}

// resolveChatMatrix turns the settings into the one identity this process can
// hold, resolving every refusal and credential before an adapter exists.
func resolveChatMatrix(config settings.Config) (chatMatrix, error) {
	m := config.Chat.Matrix
	if err := validateChatMatrix(m); err != nil {
		return chatMatrix{}, err
	}
	token, password, err := resolveCredentials(m)
	if err != nil {
		return chatMatrix{}, err
	}
	pickle, err := loadOrCreatePickleKey(m)
	if err != nil {
		return chatMatrix{}, err
	}
	recovery, err := resolveRecoveryKey(m)
	if err != nil {
		return chatMatrix{}, err
	}
	var maxAge time.Duration
	if m.MaxAge != "" {
		if maxAge, err = time.ParseDuration(m.MaxAge); err != nil {
			return chatMatrix{}, &settings.ParseError{Path: "chat.matrix.max_age", Err: err}
		}
	}
	return chatMatrix{
		homeserver: m.Homeserver, userID: m.UserID, deviceID: m.DeviceID,
		token: token, password: password, pickleKey: pickle, recoveryKey: recovery,
		allow: m.Allow, rooms: m.Rooms, maxAge: maxAge, workdir: m.Workdir,
		selfSign: settings.DerefBool(m.SelfSign),
	}, nil
}

func resolveRecoveryKey(m settings.Matrix) (string, error) {
	if m.RecoveryKey != "" {
		return m.RecoveryKey, nil
	}
	if m.RecoveryKeyCommand != "" {
		key, err := settings.KeyFromCommand(m.RecoveryKeyCommand)
		if err != nil {
			return "", &settings.ParseError{Path: "chat.matrix.recovery_key_command", Err: err}
		}
		return key, nil
	}
	return loadRecoveryKey(), nil
}
