package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/chat"
)

func (s *chatServices) cmdSnooze(_ context.Context, m chat.Message, _ string) string {
	keys := s.rooms.nudges.lastKeys(m.Room)
	if len(keys) == 0 {
		return "Rien à reporter."
	}
	until := s.snooze(keys)
	return fmt.Sprintf("Reporté à %s (%d élément(s)).", snoozeLabel(until, time.Now(), s.location()), len(keys))
}

func snoozeLabel(until, now time.Time, loc *time.Location) string {
	const day = "2006-01-02"
	if until.In(loc).Format(day) == now.In(loc).Format(day) {
		return "aujourd'hui 9h"
	}
	return "demain 9h"
}

func (s *chatServices) snooze(keys []string) time.Time {
	now := time.Now()
	until := assistant.NextNineAM(now, s.location())
	for _, key := range keys {
		if err := s.ledger.Snooze(key, now, until); err != nil {
			fmt.Fprintf(os.Stderr, "bulle chat: snooze %s: %v\n", key, err)
		}
	}
	return until
}
