package cmd

import (
	"context"
	"time"

	"github.com/FacileStudio/nacelle"

	"github.com/FacileStudio/bulle/internal/settings"
)

const dayBoundaryHour = 4

func assistantLocation(config settings.Config) *time.Location {
	loc, err := time.LoadLocation(config.Assistant.WithDefaults().Timezone)
	if err != nil {
		return time.Local
	}
	return loc
}

func (r *chatRoom) dayKey() string {
	loc := r.loc
	if loc == nil {
		loc = time.Local
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	return now().In(loc).Add(-dayBoundaryHour * time.Hour).Format("2006-01-02")
}

func (r *chatRoom) rollDay(ctx context.Context) {
	today := r.dayKey()
	if r.day == today {
		return
	}
	if r.day == "" {
		r.day = today
		return
	}
	r.newDay(ctx)
}

func (r *chatRoom) newDay(ctx context.Context) {
	conv := r.session.Conversation()
	var summary string
	if msgs := conv.Messages(); len(msgs) > 0 && r.summarize != nil {
		summary = r.summarize(ctx, msgs)
	}
	fresh := carriedMessage(summary, r.memory, r.now())
	conv.Replace(fresh)
	r.day = r.dayKey()
}

func (r *chatRoom) reset(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.newDay(ctx)
	r.persist()
}

func (r *chatRoom) record(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conv := r.session.Conversation()
	conv.AppendAssistantText(text)
	conv.CloseTurn()
	r.persist()
}

func (r *chatRoom) compactSummary(ctx context.Context, msgs []nacelle.Message) string {
	if r.compactor == nil {
		return ""
	}
	outcome := r.compactor.RunPass(ctx, msgs, true)
	if outcome.Err != nil {
		return ""
	}
	return outcome.Summary
}
