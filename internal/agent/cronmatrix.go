package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

type matrixDeliveryPayload struct {
	Action string `json:"action"`
	Room   string `json:"room"`
	Text   string `json:"text"`
}

type matrixDeliveryResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func isSilentOutput(text string) bool {
	normalized := strings.ToUpper(strings.Trim(text, " .\t\r\n`*\"'"))
	return normalized == "" || normalized == "OUTPUT NOTHING" || normalized == "NO_ALERT" || normalized == "NOTHING"
}

func chatSocketPath() (string, error) {
	if sock := os.Getenv("BULLE_CHAT_SOCK"); sock != "" {
		return sock, nil
	}
	home, err := settings.HomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving chat socket dir: %w", err)
	}
	return filepath.Join(home, "chat.sock"), nil
}

func deliverMatrix(room, text string) error {
	trimmed := strings.TrimSpace(text)
	if isSilentOutput(trimmed) {
		return nil
	}
	sockPath, err := chatSocketPath()
	if err != nil {
		return err
	}
	return sendMatrixSocket(sockPath, matrixDeliveryPayload{Action: "send", Room: room, Text: trimmed})
}

func sendMatrixSocket(sockPath string, payload matrixDeliveryPayload) error {
	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connecting to chat socket %s: %w", sockPath, err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return fmt.Errorf("setting chat socket deadline: %w", err)
	}
	if err := json.NewEncoder(conn).Encode(payload); err != nil {
		return fmt.Errorf("sending matrix delivery payload: %w", err)
	}
	var resp matrixDeliveryResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("reading matrix delivery response: %w", err)
	}
	if !resp.OK {
		if resp.Error != "" {
			return fmt.Errorf("matrix delivery failed: %s", resp.Error)
		}
		return errors.New("matrix delivery failed: response not ok")
	}
	return nil
}
