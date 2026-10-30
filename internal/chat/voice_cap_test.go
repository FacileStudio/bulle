package chat

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/bulle/internal/settings"
)

func TestReadCappedStopsAtLimit(t *testing.T) {
	if _, err := readCapped(bytes.NewReader(make([]byte, voiceMaxBytes+1))); !errors.Is(err, ErrTooLong) {
		t.Fatalf("oversize body: %v", err)
	}
	data, err := readCapped(bytes.NewReader(make([]byte, voiceMaxBytes)))
	if err != nil || len(data) != voiceMaxBytes {
		t.Fatalf("exact body: %d, %v", len(data), err)
	}
}

func TestVoiceRunCapsStdoutAndHidesStderr(t *testing.T) {
	out, err := voiceRun(context.Background(), "sh", []string{"-c", "head -c 3000000 /dev/zero"})
	if err != nil || len(out) != voiceMaxOutput {
		t.Fatalf("stdout %d bytes, err %v", len(out), err)
	}
	_, err = voiceRun(context.Background(), "sh", []string{"-c", "echo /tmp/bulle-voice-x/in >&2; exit 1"})
	if err == nil || strings.Contains(err.Error(), "bulle-voice") {
		t.Fatalf("stderr leaked: %v", err)
	}
}

func TestFfmpegArgsAreRestricted(t *testing.T) {
	args := ffmpegArgs("/t/in", "/t/out.wav")
	for _, want := range []string{"-vn", "-t", "600", "-protocol_whitelist", "file,pipe", "-nostdin"} {
		if !slices.Contains(args, want) {
			t.Errorf("missing %s in %v", want, args)
		}
	}
	if slices.Index(args, "-protocol_whitelist") > slices.Index(args, "-i") {
		t.Error("whitelist must precede -i")
	}
}

func TestTranscribeErrorHidesTempPath(t *testing.T) {
	stubRun(t, "", map[string]error{"ffmpeg": errors.New("exit status 1")})
	_, err := Transcribe(context.Background(), fakeDL{data: []byte("x")}, settings.AssistantVoice{Command: "whisper-cli -m " + fakeModel(t) + " -f {wav}"}, Media{})
	if err == nil || strings.Contains(err.Error(), "bulle-voice") {
		t.Fatalf("err = %v", err)
	}
}
