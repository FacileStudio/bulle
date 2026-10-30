package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/chat"
)

const consolidatePrefix = "memory:consolidate:"

func (s *chatServices) onReaction(ctx context.Context, m chat.Message, keys []string) bool {
	if hasPrefixKey(keys, "approval:") {
		return s.approvals.HandleReaction(ctx, m, keys)
	}
	switch m.Reaction {
	case "💤":
		s.tell(ctx, m, failureNotice("Report impossible", s.snoozeAll(keys)))
		return true
	case "👎":
		s.tell(ctx, m, failureNotice("Retour impossible", s.dislikeAll(keys)))
		return true
	}
	return false
}

func (s *chatServices) snoozeAll(keys []string) []error {
	return collect(keys, func(key string) error {
		return s.ledger.SnoozeUntilTomorrow(key, time.Now(), s.location())
	})
}

func (s *chatServices) dislikeAll(keys []string) []error {
	return collect(keys, func(key string) error {
		if date, ok := strings.CutPrefix(key, consolidatePrefix); ok {
			return s.memory.Revert(date)
		}
		return s.ledger.MarkNoise(key)
	})
}

func collect(keys []string, op func(string) error) []error {
	var errs []error
	for _, key := range keys {
		if err := op(key); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
	}
	return errs
}

func failureNotice(prefix string, errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	parts := make([]string, len(errs))
	for i, err := range errs {
		parts[i] = err.Error()
	}
	return prefix + " : " + strings.Join(parts, " ; ")
}

func (s *chatServices) tell(ctx context.Context, m chat.Message, text string) {
	if text == "" {
		return
	}
	if _, err := chat.Deliver(ctx, s.adapter, m.Identity, text, false); err != nil {
		fmt.Fprintf(os.Stderr, "bulle chat: reaction notice: %v\n", err)
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
