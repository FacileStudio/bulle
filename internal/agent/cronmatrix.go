package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
	switch normalized {
	case "", "OUTPUT NOTHING", "NO_ALERT", "NO ALERT", "NO ALERTS", "NOTHING", "NOTHING TO REPORT", "NO URGENT ITEMS":
		return true
	default:
		return false
	}
}

const matrixDeliverProtocol = "\n\nDelivery rules: only text inside <deliver></deliver> tags is sent to the chat room. " +
	"Everything outside the tags (reasoning, notes, checks) is discarded. " +
	"If nothing deserves the reader's attention, write no <deliver> block at all."

func extractDeliverable(text string) string {
	const open, closing = "<deliver>", "</deliver>"
	end := strings.LastIndex(text, closing)
	if end < 0 {
		return ""
	}
	start := strings.LastIndex(text[:end], open)
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(text[start+len(open) : end])
}

func jobPrompt(job settings.CronJob, now time.Time) string {
	prompt := formatJobPrompt(job.Prompt, now)
	if job.Delivery == "matrix" || strings.HasPrefix(job.Delivery, "matrix:") {
		prompt += matrixDeliverProtocol
	}
	return prompt
}

func chatSocketPath() (string, error) {
	sock, err := settings.ChatSocket()
	if err != nil {
		return "", fmt.Errorf("resolving chat socket dir: %w", err)
	}
	return sock, nil
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
