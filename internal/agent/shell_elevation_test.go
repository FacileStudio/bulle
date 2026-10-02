package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestShellToolDeniesElevation(t *testing.T) {
	tool := shellTestTool(t, t.TempDir(), "bash", true)
	denied := []string{
		"sudo true",
		"doas true",
		"su -c whoami",
		"pkexec true",
		"docker run alpine",
		"FOO=1 sudo true",
		"echo ok && sudo true",
		"(sudo true)",
	}
	for _, cmd := range denied {
		raw, err := json.Marshal(shellCommandInput{Command: cmd})
		if err != nil {
			t.Fatalf("marshalling: %v", err)
		}
		if _, err := tool.Run(context.Background(), raw); err == nil || !strings.Contains(err.Error(), "privilege elevation") {
			t.Fatalf("deny_elevation did not fire for %q: %v", cmd, err)
		}
	}
}

func TestShellToolAllowsSafeCommands(t *testing.T) {
	tool := shellTestTool(t, t.TempDir(), "bash", true)
	allowed := []string{
		"bulle true",
		"bulle cron list",
		"git commit -m \"add docker container\"",
		"grep -r docker .",
		"echo \"the su command\"",
		"ls -la /usr/bin/passwd",
	}
	for _, cmd := range allowed {
		raw, err := json.Marshal(shellCommandInput{Command: cmd})
		if err != nil {
			t.Fatalf("marshalling: %v", err)
		}
		if _, err := tool.Run(context.Background(), raw); err != nil && strings.Contains(err.Error(), "privilege elevation") {
			t.Fatalf("deny_elevation falsely fired for %q: %v", cmd, err)
		}
	}
}
