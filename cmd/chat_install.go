package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

// chatServiceName is the systemd user unit this command owns. One daemon per
// machine, because a Matrix account allows exactly one /sync consumer and a
// second unit sharing the account would fight it for the connection.
const chatServiceName = "bulle-chat.service"

func newChatInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Write the systemd user unit and start the daemon",
		Long: `Write ~/.config/systemd/user/bulle-chat.service, reload systemd, and
enable the unit now so it also starts at boot. Idempotent: rerunning it
rewrites the unit and leaves the service running.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runChatInstall()
		},
	}
}

func newChatUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop the daemon and remove its unit",
		Long:  "Disable and stop bulle-chat.service, then delete its unit file. The chat database and the conversation files are left in place.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runChatUninstall()
		},
	}
}

// runChatInstall prints the path and the commands before it touches anything:
// writing a unit and enabling a service mutate state outside the repository,
// and the person running it should see exactly what will happen.
func runChatInstall() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating bulle: %w", err)
	}
	workdir, err := chatWorkingDirectory()
	if err != nil {
		return err
	}
	unit := chatUnitPath()
	body := chatUnitBody(self, workdir)
	fmt.Printf("writing unit file: %s\n\n%s\n", unit, body)
	fmt.Println("running: systemctl --user daemon-reload")
	fmt.Printf("running: systemctl --user enable --now %s\n", chatServiceName)
	if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unit, []byte(body), 0o600); err != nil {
		return err
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl("enable", "--now", chatServiceName); err != nil {
		return err
	}
	fmt.Printf("bulle chat installed: %s\n", unit)
	return nil
}

func runChatUninstall() error {
	unit := chatUnitPath()
	fmt.Printf("running: systemctl --user disable --now %s\n", chatServiceName)
	if err := runSystemctl("disable", "--now", chatServiceName); err != nil {
		return err
	}
	fmt.Printf("removing unit file: %s\n", unit)
	if err := os.Remove(unit); err != nil && !os.IsNotExist(err) {
		return err
	}
	return runSystemctl("daemon-reload")
}

// chatWorkingDirectory is the run's root: the adapter's workdir when the config
// names one, and the user's home otherwise. The unit must set one, so a daemon
// that starts at boot does not inherit "/" and read it as the project root.
func chatWorkingDirectory() (string, error) {
	config, err := agent.ChatConfig()
	if err != nil {
		return "", err
	}
	if workdir := config.Chat.Matrix.Workdir; workdir != "" {
		return expandTilde(workdir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving the home directory: %w", err)
	}
	return home, nil
}

func chatUnitPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "systemd", "user", chatServiceName)
}

// chatUnitBody renders the user unit. Restart=on-failure rather than always,
// because a revoked access token fails every start: always would restart-loop
// it forever, while on-failure surfaces the failure once and stays stopped. The
// credentials file is optional, so a config that uses access_token_command
// instead of exporting secrets still starts.
func chatUnitBody(self, workdir string) string {
	return fmt.Sprintf(`[Unit]
Description=bulle chat daemon
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=%s chat
WorkingDirectory=%s
EnvironmentFile=-%s
TimeoutStartSec=60
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, self, workdir, filepath.Join(settings.ChatDir(), "env"))
}

// runSystemctl runs one systemctl --user invocation and folds its output into
// the error, so a unit systemd rejects reads as the reason rather than as a
// bare exit status.
func runSystemctl(args ...string) error {
	full := append([]string{"--user"}, args...)
	out, err := exec.Command("systemctl", full...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(full, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
