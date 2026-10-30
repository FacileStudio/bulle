package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/settings"
)

const (
	voiceMaxBytes    = 25 << 20
	voiceMaxDuration = 10 * time.Minute
	voiceTimeout     = 60 * time.Second
	voiceWavToken    = "{wav}"
	voiceDefaultCmd  = "whisper-cli -m ~/.local/share/whisper/ggml-base.bin -l auto -nt -f {wav}"
)

// ErrNoTranscriber means ffmpeg, the speech-to-text binary or its model is
// missing. The message carries the install steps, so a caller can reply with
// it verbatim.
var ErrNoTranscriber = errors.New("voice notes need ffmpeg and whisper.cpp: install ffmpeg, build whisper.cpp (whisper-cli on PATH), fetch ~/.local/share/whisper/ggml-base.bin, or set assistant.voice.command (see docs/configuration.md, \"Voice notes\")")

// ErrTooLong means the voice note is over the size or duration cap.
var ErrTooLong = errors.New("voice note is too long to transcribe (limit 25 MB or 10 minutes)")

// Downloader fetches the raw bytes of a received media file, decrypted.
type Downloader interface {
	Download(ctx context.Context, m Media) ([]byte, error)
}

// QuoteTranscript renders what was heard, for the integrator to put in front
// of the reply.
func QuoteTranscript(text string) string {
	return "🎙 « " + strings.TrimSpace(text) + " »"
}

// TooLong reports whether a media file is over 25 MB or, when dur is known
// (non-zero), over 10 minutes of audio.
func TooLong(m Media, dur time.Duration) bool {
	return m.Size > voiceMaxBytes || dur > voiceMaxDuration
}

// Transcribe downloads a voice note, converts it to 16 kHz mono wav with
// ffmpeg and returns the trimmed stdout of the configured command. All
// temporary files are removed before it returns.
func Transcribe(ctx context.Context, dl Downloader, cfg settings.AssistantVoice, m Media) (string, error) {
	if TooLong(m, 0) {
		return "", ErrTooLong
	}
	argv, err := voiceCommand(cfg.Command)
	if err != nil {
		return "", err
	}
	data, err := dl.Download(ctx, m)
	if err != nil {
		return "", fmt.Errorf("download voice note: %w", err)
	}
	if int64(len(data)) > voiceMaxBytes {
		return "", ErrTooLong
	}
	dir, err := os.MkdirTemp("", "bulle-voice-")
	if err != nil {
		return "", errors.New("prepare audio")
	}
	defer os.RemoveAll(dir)
	wav, err := voiceConvert(ctx, dir, data)
	if err != nil {
		return "", err
	}
	return voiceListen(ctx, argv, wav)
}

func voiceConvert(ctx context.Context, dir string, data []byte) (string, error) {
	in := filepath.Join(dir, "in")
	wav := filepath.Join(dir, "out.wav")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return "", errors.New("prepare audio")
	}
	ctx, cancel := context.WithTimeout(ctx, voiceTimeout)
	defer cancel()
	_, err := voiceRun(ctx, "ffmpeg", ffmpegArgs(in, wav))
	if err != nil {
		return "", voiceFailure("ffmpeg", err)
	}
	return wav, nil
}

func ffmpegArgs(in, wav string) []string {
	return []string{
		"-nostdin", "-y", "-protocol_whitelist", "file,pipe", "-i", in,
		"-vn", "-t", fmt.Sprint(int(voiceMaxDuration.Seconds())),
		"-ar", "16000", "-ac", "1", wav,
	}
}

func voiceListen(ctx context.Context, argv []string, wav string) (string, error) {
	args := make([]string, len(argv)-1)
	for i, a := range argv[1:] {
		args[i] = strings.ReplaceAll(a, voiceWavToken, wav)
	}
	ctx, cancel := context.WithTimeout(ctx, voiceTimeout)
	defer cancel()
	out, err := voiceRun(ctx, argv[0], args)
	if err != nil {
		return "", voiceFailure(argv[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

func voiceFailure(name string, err error) error {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w (missing %s)", ErrNoTranscriber, name)
	}
	return fmt.Errorf("%s: %w", name, err)
}
