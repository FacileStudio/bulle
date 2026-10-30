package cmd

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/chat"
	"github.com/FacileStudio/bulle/internal/memory"
	"github.com/FacileStudio/bulle/internal/settings"
)

type chatServices struct {
	rooms     *roomRegistry
	adapter   *chat.Matrix
	approvals *approvals
	ledger    *assistant.Ledger
	memory    *memory.Store
	assistant settings.Assistant
	stateDir  string
}

func (s *chatServices) location() *time.Location {
	loc, err := time.LoadLocation(s.assistant.WithDefaults().Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (s *chatServices) commands(reactions *chat.ReactionRouter) *chat.Commands {
	return &chat.Commands{
		New:        s.cmdNew,
		Snooze:     s.cmdSnooze,
		Quiet:      s.cmdQuiet,
		Forget:     s.cmdForget,
		Status:     s.cmdStatus,
		Audio:      s.cmdAudio,
		OnReaction: s.onReaction,
		Intercept:  s.approvals.Intercept,
		Reactions:  reactions,
	}
}

func (s *chatServices) cmdNew(ctx context.Context, m chat.Message, _ string) string {
	room, err := s.rooms.get(m.Key())
	if err != nil {
		return err.Error()
	}
	room.reset(ctx)
	return "Nouvelle conversation."
}

func (s *chatServices) cmdQuiet(_ context.Context, _ chat.Message, args string) string {
	hours, err := strconv.ParseFloat(strings.TrimSpace(args), 64)
	if err != nil || math.IsNaN(hours) || math.IsInf(hours, 0) || hours <= 0 || hours > 72 {
		return "Usage : /quiet <heures> (entre 0 et 72)."
	}
	until := time.Now().Add(time.Duration(hours * float64(time.Hour)))
	if err := assistant.SetQuiet(s.stateDir, until); err != nil {
		return "Impossible d'écrire le silence : " + err.Error()
	}
	return "Silence jusqu'à " + until.In(s.location()).Format("15:04") + "."
}

func (s *chatServices) cmdForget(_ context.Context, _ chat.Message, args string) string {
	res, err := s.memory.ForgetReport(strings.TrimSpace(args))
	if err != nil {
		return "Impossible d'oublier : " + err.Error()
	}
	reply := fmt.Sprintf("Oublié : %d ligne(s).", res.Removed)
	if len(res.Backups) > 0 {
		reply += " Sauvegardes (backups/) : " + strings.Join(res.Backups, ", ")
	}
	return reply
}

func (s *chatServices) cmdStatus(_ context.Context, _ chat.Message, _ string) string {
	now := time.Now()
	lines := []string{
		fmt.Sprintf("Dernière veille : %s", assistant.LastWatch(s.stateDir).In(s.location()).Format("02/01 15:04")),
		fmt.Sprintf("Éléments suivis : %d", s.ledger.Len()),
		fmt.Sprintf("Mémoire : %d caractères", len(s.memory.Snapshot(now))),
	}
	if assistant.InQuiet(now, s.assistant, s.stateDir) {
		lines = append(lines, "Mode silencieux actif.")
	}
	return strings.Join(lines, "\n")
}
