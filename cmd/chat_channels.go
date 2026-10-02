package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FacileStudio/bulle/internal/agent"
	"github.com/FacileStudio/bulle/internal/settings"
	"github.com/spf13/cobra"
)

// chatChannelJSON is one configured adapter as the JSON document renders it.
type chatChannelJSON struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Homeserver string `json:"homeserver"`
	UserID     string `json:"user_id"`
	Senders    int    `json:"senders"`
	Rooms      int    `json:"rooms"`
	Store      string `json:"store"`
	StoreReady bool   `json:"store_ready"`
	SelfSign   bool   `json:"self_sign"`
}

// newChatChannelsCmd lists the configured adapters and their state, without
// opening any of them: it reads the settings and stats the store, so it works
// on a machine where the daemon is stopped or has never run.
func newChatChannelsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "List configured chat adapters and their state",
		Long:  "List every configured chat adapter with its homeserver, identity, allowlist size and crypto store.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runChatChannels(asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print adapters as a JSON document")
	return cmd
}

func runChatChannels(asJSON bool) error {
	config, err := agent.ChatConfig()
	if err != nil {
		return err
	}
	m := config.Chat.Matrix
	store := settings.StorePath()
	_, statErr := os.Stat(store)
	channels := []chatChannelJSON{{
		Name: "matrix", Enabled: settings.DerefBool(m.Enabled),
		Homeserver: m.Homeserver, UserID: m.UserID,
		Senders: len(m.Allow), Rooms: len(m.Rooms),
		Store: store, StoreReady: statErr == nil,
		SelfSign: settings.DerefBool(m.SelfSign),
	}}
	if asJSON || settings.DerefBool(config.JSON) {
		data, err := json.Marshal(channels)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	for _, c := range channels {
		state := "disabled"
		if c.Enabled {
			state = "enabled"
		}
		fmt.Printf("matrix [%s]\nhomeserver=%s\nuser_id=%s\nsenders=%d\nrooms=%d\nself_sign=%t\nstore=%s\nstore_ready=%t\n",
			state, c.Homeserver, c.UserID, c.Senders, c.Rooms, c.SelfSign, c.Store, c.StoreReady)
	}
	return nil
}
