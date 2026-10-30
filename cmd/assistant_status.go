package cmd

import (
	"fmt"
	"time"

	"github.com/FacileStudio/bulle/internal/assistant"
	"github.com/FacileStudio/bulle/internal/settings"
)

func printAssistantStatus() error {
	cfg, err := assistantSettings()
	if err != nil {
		return err
	}
	state, err := settings.StateDir()
	if err != nil {
		return err
	}
	ledger, err := assistant.NewLedger("")
	if err != nil {
		return err
	}
	last := "never"
	if t := assistant.LastWatch(state); !t.IsZero() {
		last = t.Local().Format(time.RFC3339)
	}
	fmt.Printf("last watch: %s\nledger entries: %d\n", last, ledger.Len())
	now := time.Now()
	for _, name := range assistant.DigestNames() {
		next := assistant.NextDigest(cfg, name, now)
		if next.IsZero() {
			fmt.Printf("next %s: none\n", name)
			continue
		}
		fmt.Printf("next %s: %s\n", name, next.Format("Mon 2006-01-02 15:04 MST"))
	}
	return nil
}
