package cmd

import (
	"context"
	"strings"

	"github.com/FacileStudio/bulle/internal/chat"
)

const consolidatePrefix = "memory:consolidate:"

func (s *chatServices) onReaction(ctx context.Context, m chat.Message, keys []string) bool {
	if hasPrefixKey(keys, "approval:") {
		return s.approvals.HandleReaction(ctx, m, keys)
	}
	switch m.Reaction {
	case "💤":
		s.snooze(keys)
		return true
	case "👎":
		s.dislike(keys)
		return true
	}
	return false
}

func (s *chatServices) dislike(keys []string) {
	for _, key := range keys {
		if date, ok := strings.CutPrefix(key, consolidatePrefix); ok {
			_ = s.memory.Revert(date)
			continue
		}
		_ = s.ledger.MarkNoise(key)
	}
}

func hasPrefixKey(keys []string, prefix string) bool {
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

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
		return err.Error()
	}
	reply, err := room.answer(ctx, text)
	if err != nil {
		return chat.QuoteTranscript(text)
	}
	return chat.QuoteTranscript(text) + "\n\n" + reply
}
