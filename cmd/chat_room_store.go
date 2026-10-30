package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

type roomEntry struct {
	Day  string `json:"day,omitempty"`
	Role string `json:"role,omitempty"`
	Text string `json:"text,omitempty"`
}

func roomStorePath(key string) (string, error) {
	home, err := settings.HomeDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:])[:16] + ".jsonl"
	return filepath.Join(home, "chat", "rooms", name), nil
}

func loadRoom(path string) (string, []nacelle.Message) {
	file, err := os.Open(path)
	if path == "" || err != nil {
		return "", nil
	}
	defer file.Close()
	var day string
	var msgs []nacelle.Message
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scan.Scan() {
		var e roomEntry
		if json.Unmarshal(scan.Bytes(), &e) != nil {
			continue
		}
		if e.Day != "" {
			day = e.Day
		}
		if e.Role != "" && e.Text != "" {
			msgs = append(msgs, nacelle.Message{Role: nacelle.Role(e.Role), Parts: []nacelle.Part{nacelle.Text{Text: e.Text}}})
		}
	}
	return day, userFirst(msgs)
}

func roomText(m nacelle.Message) string {
	var b strings.Builder
	for _, part := range m.Parts {
		if t, ok := part.(nacelle.Text); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

func encodeRoom(day string, msgs []nacelle.Message) []byte {
	line, _ := json.Marshal(roomEntry{Day: day})
	out := append(line, '\n')
	for _, m := range msgs {
		text := roomText(m)
		if text == "" {
			continue
		}
		line, _ = json.Marshal(roomEntry{Role: string(m.Role), Text: text})
		out = append(append(out, line...), '\n')
	}
	return out
}

func saveRoom(path, day string, msgs []nacelle.Message) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".room-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(encodeRoom(day, msgs)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (r *chatRoom) persist() {
	if r.path == "" || r.session == nil {
		return
	}
	if err := saveRoom(r.path, r.day, r.session.Conversation().Messages()); err != nil {
		fmt.Fprintf(os.Stderr, "bulle chat: persist room: %v\n", err)
	}
}
