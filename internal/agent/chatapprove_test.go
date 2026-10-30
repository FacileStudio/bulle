package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/FacileStudio/bulle/internal/xdgtest"
)

const testHome = "/home/u"

const j = "~/Documents/NuagePersonal/Wiki/Journal/2026-10-30.md"

func checkReadOnly(t *testing.T, cases map[string]bool) {
	t.Helper()
	xdgtest.Isolate(t, testHome)
	for cmd, want := range cases {
		if got := readOnlyIn(testHome, cmd); got != want {
			t.Errorf("readOnly(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestReadOnlyCourrier(t *testing.T) {
	cases := map[string]bool{
		"courrier inbox":                              true,
		"courrier inbox --unread --json":              true,
		"courrier search \"facture edf\"":             true,
		"courrier accounts":                           true,
		"courrier accounts add x":                     false,
		"courrier contacts delete 3":                  false,
		"courrier triage --all --json":                true,
		"courrier send --to a@b --subject s --body x": false,
		"courrier inbox; rm -rf ~":                    false,
		"courrier inbox | head":                       false,
		"courrier inbox && ls":                        false,
		"courrier inbox `id`":                         false,
		"courrier inbox $(id)":                        false,
		"courrier inbox > /tmp/x":                     false,
		"courrier inbox\nrm x":                        false,
	}
	checkReadOnly(t, cases)
}

func TestReadOnlyTools(t *testing.T) {
	cases := map[string]bool{
		"agenda calendars list":                   true,
		"agenda events list --json":               true,
		"agenda events create x":                  false,
		"opus workspace list":                     true,
		"opus -w 3 task list":                     true,
		"opus -w 3 task show 9":                   true,
		"opus -w 3 task delete 9":                 false,
		"mycelium memory search foo":              true,
		"mycelium memory add x":                   false,
		"date":                                    true,
		"date +%H:%M":                             true,
		"date -s 2020-01-01":                      false,
		"cat ~/.local/share/bulle/memory/USER.md": true,
		"ls -la ~/.local/share/bulle/memory":      true,
		"cat /etc/passwd":                         false,
	}
	checkReadOnly(t, cases)
}

func TestReadOnlyPaths(t *testing.T) {
	cases := map[string]bool{
		"cat ~/.local/share/bulle/memory/../../.ssh/id_ed25519": false,
		"cat USER.md": false,
		"ls":          false,
		"cat ~/.local/share/bulle/memory/USER.md ~/.ssh/id_rsa":         false,
		"echo 'note du jour' >> " + j:                                   true,
		"echo \"note\" >> " + j:                                         true,
		"echo note >> " + j:                                             true,
		"echo 'x' >> ~/other.md":                                        false,
		"echo 'x' >> ~/Documents/NuagePersonal/Wiki/Journal/../../x.md": false,
		"echo \"$(id)\" >> " + j:                                        false,
		"echo 'x' >> " + j + "; rm -rf ~":                               false,
		"echo 'x' > " + j:                                               false,
		"rm -rf ~":                                                      false,
		"":                                                              false,
		"cat 'unterminated":                                             false,
	}
	checkReadOnly(t, cases)
}

type fakeAsker struct {
	answer bool
	err    error
	asked  []string
}

func (f *fakeAsker) Ask(_ context.Context, _ string, summary string) (bool, error) {
	f.asked = append(f.asked, summary)
	return f.answer, f.err
}

func input(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestChatApprove(t *testing.T) {
	ctx := context.Background()
	asker := &fakeAsker{answer: true}
	gate := ChatApprove("matrix:!r", asker, false)
	if !gate(ctx, "read_file", input(map[string]string{"path": "a"})) {
		t.Error("read_file should be free")
	}
	if !gate(ctx, "run_command", input(map[string]string{"command": "date"})) {
		t.Error("date should be free")
	}
	if len(asker.asked) != 0 {
		t.Fatalf("asked too early: %v", asker.asked)
	}
	if !gate(ctx, "run_command", input(map[string]string{"command": "courrier send --to a@b"})) {
		t.Error("approved command should run")
	}
	if !gate(ctx, "write_file", input(map[string]string{"path": "a"})) {
		t.Error("approved write_file should run")
	}
	if len(asker.asked) != 2 {
		t.Fatalf("asked = %v", asker.asked)
	}
}

func TestChatApproveDenies(t *testing.T) {
	ctx := context.Background()
	cmd := input(map[string]string{"command": "rm x"})
	if ChatApprove("r", &fakeAsker{answer: false}, false)(ctx, "run_command", cmd) {
		t.Error("denied answer must deny")
	}
	if ChatApprove("r", &fakeAsker{answer: true, err: errors.New("boom")}, false)(ctx, "run_command", cmd) {
		t.Error("asker error must deny")
	}
	if ChatApprove("r", nil, false)(ctx, "run_command", cmd) {
		t.Error("nil asker must deny")
	}
	dup := json.RawMessage(`{"command":"date","command":"rm x"}`)
	if ChatApprove("r", &fakeAsker{answer: true}, false)(ctx, "run_command", dup) {
		t.Error("duplicate keys must deny")
	}
}

func TestChatApproveAutoSkipsTheQuestionButNotTheForgeryCheck(t *testing.T) {
	ctx := context.Background()
	asker := &fakeAsker{answer: false}
	gate := ChatApprove("r", asker, true)
	if !gate(ctx, "run_command", json.RawMessage(`{"command":"rm x"}`)) {
		t.Error("auto must approve a non-read command")
	}
	if len(asker.asked) != 0 {
		t.Errorf("auto must not ask, asked %v", asker.asked)
	}
	if gate(ctx, "run_command", json.RawMessage(`{"command":"date","command":"rm x"}`)) {
		t.Error("duplicate keys must still deny")
	}
}
