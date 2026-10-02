package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

// newChatVerifyCmd verifies the bot device using cross-signing keys.
func newChatVerifyCmd() *cobra.Command {
	var recoveryKey string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the Matrix bot device with cross-signing",
		Long: `Verify this bot device using your Matrix account's recovery key.

With --recovery-key (or recovery_key set in ~/.bulle.yml), bulle
fetches the cross-signing keys from SSSS and signs the device directly.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runChatVerify(cmd.Context(), recoveryKey)
		},
	}
	cmd.Flags().StringVar(&recoveryKey, "recovery-key", "", "Account recovery key (from Element Security settings)")
	return cmd
}

func runChatVerify(ctx context.Context, recoveryKey string) error {
	config, err := agent.ChatConfig()
	if err != nil {
		return err
	}
	m, err := resolveChatMatrix(config)
	if err != nil {
		return err
	}
	key := pickRecoveryKey(m, recoveryKey)
	adapter, cleanup, err := openVerifyAdapter(m)
	if err != nil {
		return err
	}
	defer cleanup()
	return executeVerify(ctx, adapter, key)
}

func pickRecoveryKey(m chatMatrix, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if m.recoveryKey != "" {
		return m.recoveryKey
	}
	return promptRecoveryKey()
}

func promptRecoveryKey() string {
	fmt.Print("Enter Matrix recovery key (from Element Security settings): ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

func openVerifyAdapter(m chatMatrix) (*chat.Matrix, func(), error) {
	db, err := chat.OpenStore(settings.StorePath())
	if err != nil {
		return nil, nil, err
	}
	adapter, err := chat.NewMatrix(chat.MatrixConfig{
		Homeserver: m.homeserver, UserID: m.userID, DeviceID: m.deviceID,
		Token: m.token, Password: m.password, PickleKey: m.pickleKey, Database: db,
		SelfSign: false,
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, nil, errors.Join(err, closeErr)
		}
		return nil, nil, err
	}
	cleanup := func() {
		if closeErr := adapter.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "bulle chat: closing adapter: %v\n", closeErr)
		}
	}
	return adapter, cleanup, nil
}

func executeVerify(ctx context.Context, adapter *chat.Matrix, key string) error {
	_, isVerified, err := adapter.VerificationStatus(ctx)
	if err != nil {
		return fmt.Errorf("checking verification status: %w", err)
	}
	if isVerified {
		fmt.Println("bulle chat: device is already verified")
		return nil
	}
	if key == "" {
		return errors.New("bulle chat: no recovery key provided; set recovery_key in config or pass --recovery-key")
	}
	if err := adapter.VerifyWithRecoveryKey(ctx, key); err != nil {
		return fmt.Errorf("verifying with recovery key: %w", err)
	}
	if err := saveRecoveryKey(key); err != nil {
		return fmt.Errorf("saving recovery key: %w", err)
	}
	fmt.Println("bulle chat: device verified successfully with recovery key")
	return nil
}
