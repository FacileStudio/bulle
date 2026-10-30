package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/FacileStudio/bulle/internal/chat"
)

func (s *chatServices) cmdAudio(ctx context.Context, m chat.Message, _ string) string {
	if m.Media == nil {
		return ""
	}
	text, err := chat.Transcribe(ctx, s.adapter, s.assistant.Voice, *m.Media)
	if err != nil {
		return err.Error()
	}
	room, err := s.rooms.get(m.Key())
	if err != nil {
		return audioReply(text, "", err)
	}
	reply, err := room.answer(ctx, text)
	return audioReply(text, reply, err)
}

func audioReply(text, reply string, err error) string {
	quote := chat.QuoteTranscript(text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bulle chat: audio turn: %v\n", err)
		line := strings.Join(strings.Fields(err.Error()), " ")
		return quote + "\n\nLa réponse a échoué : " + line
	}
	return quote + "\n\n" + reply
}
