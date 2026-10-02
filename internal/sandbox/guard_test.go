package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildGuardProbeCommand(t *testing.T) {
	cmd1 := BuildGuardProbeCommand("")
	if !strings.Contains(cmd1, "whoami") || !strings.Contains(cmd1, "pwd") {
		t.Fatalf("unexpected probe command: %s", cmd1)
	}
	cmd2 := BuildGuardProbeCommand("/workspace")
	if !strings.Contains(cmd2, "WORKDIR_OK") || !strings.Contains(cmd2, "/workspace") {
		t.Fatalf("unexpected probe command with workdir: %s", cmd2)
	}
}

func TestParseGuardOutput(t *testing.T) {
	output := "boite\n/home/boite\nWORKDIR_OK\n"
	res, err := ParseGuardOutput(output)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if res.User != "boite" || res.Workdir != "/home/boite" {
		t.Fatalf("unexpected guard result: %+v", res)
	}
	if _, err := ParseGuardOutput("single_line"); err == nil {
		t.Fatal("expected error for insufficient lines")
	}
}

func TestVerifyIsolation(t *testing.T) {
	opts := GuardOptions{ExpectedUser: "boite", ExpectedWorkdir: "/workspace"}
	res := &GuardResult{User: "root", Workdir: "/root"}
	if err := VerifyIsolation(res, "WORKDIR_OK", opts); err == nil {
		t.Fatal("expected error when root does not match the expected user")
	}
	if err := VerifyIsolation(res, "WORKDIR_OK", GuardOptions{ExpectedUser: "root"}); err != nil {
		t.Fatalf("expected root to be allowed when the target asks for it: %v", err)
	}
	if err := VerifyIsolation(res, "WORKDIR_OK", GuardOptions{}); err != nil {
		t.Fatalf("expected root to be allowed with no expectation set: %v", err)
	}
	res.User = "other"
	if err := VerifyIsolation(res, "WORKDIR_OK", opts); err == nil {
		t.Fatal("expected error for user mismatch")
	}
	res.User = "boite"
	if err := VerifyIsolation(res, "WORKDIR_MISSING", opts); err == nil {
		t.Fatal("expected error for missing workdir")
	}
	if err := VerifyIsolation(res, "WORKDIR_OK", opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreflightCheck(t *testing.T) {
	target := &Target{Name: "pingu", Backend: "boite", Port: 2226, KeyPath: "/key", Status: "stopped"}
	opts := DefaultGuardOptions()
	if _, err := PreflightCheck(context.Background(), target, opts); err == nil {
		t.Fatal("expected error for stopped VM")
	}
	target.Status = "running"
	opts.Runner = &mockRunner{
		runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("boite\n/home/boite\n"), nil
		},
	}
	res, err := PreflightCheck(context.Background(), target, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.User != "boite" {
		t.Fatalf("expected boite user, got %s", res.User)
	}
}

// BatchMode means ssh cannot prompt to trust an unknown host, so the probe has
// to turn the raw exit status into a command the user can run.
func TestPreflightCheckHostKeyHint(t *testing.T) {
	target := &Target{Name: "boite@localhost:2226", Backend: "ssh", Host: "localhost", Port: 2226, User: "boite", Status: "running"}
	opts := GuardOptions{
		Runner: &mockRunner{
			runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("Host key verification failed."), errors.New("exit status 255")
			},
		},
	}
	_, err := PreflightCheck(context.Background(), target, opts)
	if err == nil {
		t.Fatal("expected a host key failure")
	}
	for _, want := range []string{"not in ~/.ssh/known_hosts", "ssh -p 2226 boite@localhost", "ssh-keyscan -p 2226 localhost", "bulle sandbox"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in error, got %v", want, err)
		}
	}
}

// ssh-keyscan does not read ssh_config, so an alias must be left to ssh — the
// one command that resolves the real host and port — rather than scanned under
// a name that only ssh understands.
func TestPreflightCheckHostKeyHintKeepsAliasToSSH(t *testing.T) {
	target := &Target{Name: "myserver", Backend: "ssh", Host: "myserver", User: "deploy", Status: "running"}
	opts := GuardOptions{
		Runner: &mockRunner{
			runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("Host key verification failed."), errors.New("exit status 255")
			},
		},
	}
	_, err := PreflightCheck(context.Background(), target, opts)
	if err == nil {
		t.Fatal("expected a host key failure")
	}
	if strings.Contains(err.Error(), "ssh-keyscan") {
		t.Fatalf("an ssh_config alias must not get an ssh-keyscan hint: %v", err)
	}
	if !strings.Contains(err.Error(), "ssh deploy@myserver") {
		t.Fatalf("expected the ssh command for the alias, got %v", err)
	}
}

func TestPreflightCheckHostKeyChangedHint(t *testing.T) {
	target := &Target{Name: "host", Backend: "ssh", Host: "build.example.com", Port: 22, User: "deploy", Status: "running"}
	opts := GuardOptions{
		Runner: &mockRunner{
			runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("REMOTE HOST IDENTIFICATION HAS CHANGED!"), errors.New("exit status 255")
			},
		},
	}
	_, err := PreflightCheck(context.Background(), target, opts)
	if err == nil || !strings.Contains(err.Error(), "ssh-keygen -R [build.example.com]:22") {
		t.Fatalf("expected the stale-entry hint, got %v", err)
	}
}

func TestPreflightCheckFailures(t *testing.T) {
	if _, err := PreflightCheck(context.Background(), nil, DefaultGuardOptions()); err == nil {
		t.Fatal("expected error for nil instance")
	}
	target := &Target{Name: "pingu", Backend: "boite", Port: 2226, KeyPath: "/key", Status: "running"}
	opts := GuardOptions{
		Runner: &mockRunner{
			runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return nil, errors.New("connection refused")
			},
		},
	}
	if _, err := PreflightCheck(context.Background(), target, opts); err == nil {
		t.Fatal("expected error when ssh runner fails")
	}
}
