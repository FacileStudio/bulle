package chat

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

type fakeDL struct {
	data []byte
}

func (f fakeDL) Download(context.Context, Media) ([]byte, error) { return f.data, nil }

type voiceCall struct {
	name string
	args []string
}

func stubRun(t *testing.T, out string, fail map[string]error) *[]voiceCall {
	t.Helper()
	var calls []voiceCall
	old := voiceRun
	t.Cleanup(func() { voiceRun = old })
	voiceRun = func(_ context.Context, name string, args []string) ([]byte, error) {
		calls = append(calls, voiceCall{name, args})
		if err := fail[name]; err != nil {
			return nil, err
		}
		return []byte(out), nil
	}
	return &calls
}

func fakeModel(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "model.bin")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTranscribeTrimsAndSubstitutes(t *testing.T) {
	calls := stubRun(t, "  hello world \n", nil)
	cfg := settings.AssistantVoice{Command: "stt -m " + fakeModel(t) + " --file={wav} -f {wav}"}
	got, err := Transcribe(context.Background(), fakeDL{data: []byte("ogg")}, cfg, Media{Size: 3})
	if err != nil || got != "hello world" {
		t.Fatalf("got %q, %v", got, err)
	}
	ff, stt := (*calls)[0], (*calls)[1]
	wav := ff.args[len(ff.args)-1]
	if ff.name != "ffmpeg" || !strings.HasSuffix(wav, "out.wav") {
		t.Fatalf("ffmpeg call %v", ff)
	}
	if stt.args[len(stt.args)-1] != wav || !strings.Contains(strings.Join(stt.args, " "), "--file="+wav) {
		t.Fatalf("wav not substituted: %v", stt.args)
	}
	if _, err := os.Stat(filepath.Dir(wav)); !os.IsNotExist(err) {
		t.Fatalf("temp dir left behind: %v", err)
	}
}

func TestTranscribeMissingBinary(t *testing.T) {
	stubRun(t, "", map[string]error{"stt": exec.ErrNotFound})
	cfg := settings.AssistantVoice{Command: "stt {wav}"}
	_, err := Transcribe(context.Background(), fakeDL{data: []byte("x")}, cfg, Media{})
	if !errors.Is(err, ErrNoTranscriber) || !strings.Contains(err.Error(), "install") {
		t.Fatalf("got %v", err)
	}
}

func TestTranscribeMissingModel(t *testing.T) {
	calls := stubRun(t, "x", nil)
	cfg := settings.AssistantVoice{Command: "stt -m /nonexistent/model.bin {wav}"}
	_, err := Transcribe(context.Background(), fakeDL{data: []byte("x")}, cfg, Media{})
	if !errors.Is(err, ErrNoTranscriber) || len(*calls) != 0 {
		t.Fatalf("got %v, calls %v", err, *calls)
	}
}

func TestTranscribeOversize(t *testing.T) {
	calls := stubRun(t, "x", nil)
	_, err := Transcribe(context.Background(), fakeDL{}, settings.AssistantVoice{}, Media{Size: voiceMaxBytes + 1})
	if !errors.Is(err, ErrTooLong) || len(*calls) != 0 {
		t.Fatalf("got %v", err)
	}
	if !TooLong(Media{}, 11*time.Minute) || TooLong(Media{}, 0) {
		t.Fatal("duration guard wrong")
	}
}

func TestSplitWords(t *testing.T) {
	got, err := splitWords(`a "b c" 'd e' f""g`)
	if err != nil || strings.Join(got, "|") != "a|b c|d e|fg" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := splitWords(`a "b`); err == nil {
		t.Fatal("unterminated quote accepted")
	}
}

func TestQuoteTranscript(t *testing.T) {
	if got := QuoteTranscript(" salut "); got != "🎙 « salut »" {
		t.Fatalf("got %q", got)
	}
}
