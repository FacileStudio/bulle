package agent

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/FacileStudio/nacelle"
)

func TestCronRunRecordCarriesStatusAndError(t *testing.T) {
	stats := runStats{
		Usage:              nacelle.Usage{InputTokens: 100, OutputTokens: 20, Cost: 0.5},
		ToolCalls:          3,
		FinalContextTokens: 120,
		Compactions:        2,
	}
	started := time.Unix(0, 0)
	ok, _ := stats.record(settings.CronJob{Name: "job"}, "model-x", started, nil)
	if ok.Status != string(cronOK) || ok.Error != "" || ok.Usage.Cost != 0.5 || ok.Compactions != 2 {
		t.Fatalf("clean run record wrong: %+v", ok)
	}
	bad, status := stats.record(settings.CronJob{Name: "job"}, "model-x", started, errors.New("boom"))
	if status != cronFailed || bad.Status != "failed" || bad.Error != "boom" {
		t.Fatalf("failed run record wrong: %+v", bad)
	}
}

func TestAppendCronRunWritesOneJSONLine(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	rec, _ := (runStats{Usage: nacelle.Usage{OutputTokens: 7}}).record(settings.CronJob{Name: "job"}, "m", time.Unix(0, 0), nil)
	if err := appendCronRun(rec); err != nil {
		t.Fatal(err)
	}
	path, err := cronStatePath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one JSON line, got %d", len(lines))
	}
	var got cronRun
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "job" || got.Usage.OutputTokens != 7 {
		t.Fatalf("round-trip lost data: %+v", got)
	}
}

func TestValidateDeliveryFormats(t *testing.T) {
	valid := []string{"", "file:/tmp/log", "file:./cron", "matrix", "matrix:!abc:matrix.org", "matrix:"}
	for _, v := range valid {
		if err := validateDelivery(v); err != nil {
			t.Errorf("validateDelivery(%q) = %v; want nil", v, err)
		}
	}
	invalid := []string{"webhook:http://example.com", "slack:#general", "stdout"}
	for _, inv := range invalid {
		err := validateDelivery(inv)
		if err == nil {
			t.Errorf("validateDelivery(%q) = nil; want error", inv)
		} else if !strings.Contains(err.Error(), "want file:<dir> or matrix[:<room>]") {
			t.Errorf("validateDelivery(%q) error = %q; want expected message", inv, err.Error())
		}
	}
}

func mockMatrixSocket(t *testing.T, dir string, handler func(matrixDeliveryPayload) matrixDeliveryResponse) string {
	t.Helper()
	sockPath := filepath.Join(dir, "chat.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			return
		}
	})
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		var req matrixDeliveryPayload
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			return
		}
		resp := handler(req)
		if err := json.NewEncoder(conn).Encode(resp); err != nil {
			return
		}
	}()
	return sockPath
}

func TestAppendCronLogDeliversViaMatrixSocket(t *testing.T) {
	dir := t.TempDir()
	received := make(chan matrixDeliveryPayload, 1)
	sockPath := mockMatrixSocket(t, dir, func(req matrixDeliveryPayload) matrixDeliveryResponse {
		received <- req
		return matrixDeliveryResponse{OK: true}
	})
	t.Setenv("BULLE_CHAT_SOCK", sockPath)
	if err := appendCronLog("matrix:!abc:matrix.org", "daily", cronOK, "<deliver>daily done</deliver>"); err != nil {
		t.Fatalf("appendCronLog failed: %v", err)
	}
	req := <-received
	if req.Action != "send" || req.Room != "!abc:matrix.org" || req.Text != "daily done" {
		t.Errorf("unexpected payload: %+v", req)
	}
}

func TestAppendCronLogDeliversMatrixWithoutRoom(t *testing.T) {
	dir := t.TempDir()
	received := make(chan matrixDeliveryPayload, 1)
	sockPath := mockMatrixSocket(t, dir, func(req matrixDeliveryPayload) matrixDeliveryResponse {
		received <- req
		return matrixDeliveryResponse{OK: true}
	})
	t.Setenv("BULLE_CHAT_SOCK", sockPath)
	if err := appendCronLog("matrix", "daily", cronOK, "reasoning <deliver>job output</deliver>"); err != nil {
		t.Fatalf("appendCronLog failed: %v", err)
	}
	req := <-received
	if req.Action != "send" || req.Room != "" || req.Text != "job output" {
		t.Errorf("unexpected payload: %+v", req)
	}
}

func TestDeliverMatrixSocketError(t *testing.T) {
	dir := t.TempDir()
	sockPath := mockMatrixSocket(t, dir, func(req matrixDeliveryPayload) matrixDeliveryResponse {
		return matrixDeliveryResponse{OK: false, Error: "room not joined"}
	})
	t.Setenv("BULLE_CHAT_SOCK", sockPath)
	err := deliverMatrix("!bad:matrix.org", "text")
	if err == nil || !strings.Contains(err.Error(), "room not joined") {
		t.Fatalf("expected error mentioning 'room not joined', got %v", err)
	}

	t.Setenv("BULLE_CHAT_SOCK", filepath.Join(dir, "nonexistent.sock"))
	if err := deliverMatrix("", "text"); err == nil {
		t.Fatal("expected connection error for missing socket, got nil")
	}
}

func TestAppendCronLogMatrixFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BULLE_CHAT_SOCK", filepath.Join(home, "nonexistent.sock"))

	if err := appendCronLog("matrix:!bad:matrix.org", "daily", cronOK, "<deliver>fallback text</deliver>"); err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}

	logPath := filepath.Join(home, ".bulle", "logs", "delivery-fallback.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading fallback log: %v", err)
	}
	if !strings.Contains(string(data), string(cronOK)) {
		t.Errorf("expected %q in fallback log, got %s", cronOK, string(data))
	}

	today := time.Now().Format("2006-01-02")
	mdPath := filepath.Join(home, ".bulle", "logs", "delivery-fallback-"+today+".md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("reading fallback md: %v", err)
	}
	if !strings.Contains(string(mdData), "fallback text") {
		t.Errorf("expected 'fallback text' in fallback md, got %s", string(mdData))
	}
}

func TestIsSilentOutput(t *testing.T) {
	if !isSilentOutput("") {
		t.Fatal("empty string must be silent")
	}
	if !isSilentOutput("\n\n\r\t") {
		t.Fatal("whitespace string must be silent")
	}
	if !isSilentOutput("OUTPUT NOTHING.") {
		t.Fatal("OUTPUT NOTHING. must be silent")
	}
	if !isSilentOutput("\n\nOUTPUT NOTHING\n") {
		t.Fatal("newlines with OUTPUT NOTHING must be silent")
	}
	if !isSilentOutput("NO_ALERT") {
		t.Fatal("NO_ALERT must be silent")
	}
	if !isSilentOutput("NO ALERT") {
		t.Fatal("NO ALERT must be silent")
	}
	if !isSilentOutput("NO ALERTS") {
		t.Fatal("NO ALERTS must be silent")
	}
	if !isSilentOutput("NOTHING TO REPORT") {
		t.Fatal("NOTHING TO REPORT must be silent")
	}
	if !isSilentOutput("NO URGENT ITEMS") {
		t.Fatal("NO URGENT ITEMS must be silent")
	}
	if isSilentOutput("real content") {
		t.Fatal("real content must not be silent")
	}
}

func TestAppendCronLogMatrixSilentSkipsDelivery(t *testing.T) {
	t.Setenv("BULLE_CHAT_SOCK", filepath.Join(t.TempDir(), "nonexistent.sock"))
	if err := appendCronLog("matrix", "daily", cronOK, "OUTPUT NOTHING."); err != nil {
		t.Fatalf("expected silent skip without contacting socket, got %v", err)
	}
	if err := appendCronLog("matrix", "daily", cronOK, "\n\n\n"); err != nil {
		t.Fatalf("expected whitespace skip without contacting socket, got %v", err)
	}
}
