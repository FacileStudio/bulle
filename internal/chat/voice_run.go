package chat

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

const voiceMaxOutput = 1 << 20

type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

var voiceRun = func(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out := &cappedBuffer{max: voiceMaxOutput}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return out.buf.Bytes(), err
}

func readCapped(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, voiceMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > voiceMaxBytes {
		return nil, ErrTooLong
	}
	return data, nil
}
