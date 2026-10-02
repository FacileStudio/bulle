package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const chatConfig = `
chat:
  matrix:
    enabled: true
    homeserver: https://matrix.example.org
    user_id: "@bulle-bot:example.org"
    self_sign: true
    allow:
      - "@alice:example.org"
    rooms:
      - "!room:example.org"
    max_age: 2h
    workdir: ~/Code
`

// TestTheChatBlockLoads proves every key the scaffold documents is one the
// loader accepts. Load decodes with KnownFields, so a key the struct does not
// carry is a startup failure rather than a setting that silently does nothing,
// and the scaffold is what a reader copies from.
func TestTheChatBlockLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bulle.yml")
	if err := os.WriteFile(path, []byte(chatConfig), 0o644); err != nil {
		t.Fatalf("writing tmp config: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	m := loaded.Chat.Matrix
	if !DerefBool(m.Enabled) {
		t.Error("enabled = false, want true")
	}
	if m.Homeserver != "https://matrix.example.org" || m.UserID != "@bulle-bot:example.org" {
		t.Errorf("homeserver/user_id = %q/%q", m.Homeserver, m.UserID)
	}
	if !DerefBool(m.SelfSign) {
		t.Error("self_sign = false, want true")
	}
	if len(m.Allow) != 1 || m.Allow[0] != "@alice:example.org" {
		t.Errorf("allow = %v, want the one MXID", m.Allow)
	}
	if len(m.Rooms) != 1 || m.Rooms[0] != "!room:example.org" {
		t.Errorf("rooms = %v, want the one room", m.Rooms)
	}
	if m.MaxAge != "2h" || m.Workdir != "~/Code" {
		t.Errorf("max_age/workdir = %q/%q", m.MaxAge, m.Workdir)
	}
}

// TestChatIsOffAndUnsupervisedByDefault pins the two defaults that are security
// decisions rather than conveniences: a fresh install opens no connection, and
// the bot publishes no cross-signing identity behind the operator's back.
func TestChatIsOffAndUnsupervisedByDefault(t *testing.T) {
	defaults := Defaults("")
	if DerefBool(defaults.Chat.Matrix.Enabled) {
		t.Error("chat.matrix.enabled defaults to true, want an untouched install to open no connection")
	}
	if DerefBool(defaults.Chat.Matrix.SelfSign) {
		t.Error("chat.matrix.self_sign defaults to true, want self-signing to be asked for")
	}
	if defaults.Chat.Matrix.Homeserver != "" || len(defaults.Chat.Matrix.Allow) != 0 {
		t.Errorf("chat defaults carry a homeserver or an allowlist: %+v", defaults.Chat.Matrix)
	}
}

// TestALayerCanTurnChatOnAndBackOff proves self_sign merges like every other
// toggle. The false case is the one worth pinning: a merge that only ever sets
// fields to true, or that treats false as absent, leaves a layer unable to turn
// self-signing off, which is the direction that matters.
func TestALayerCanTurnChatOnAndBackOff(t *testing.T) {
	on, off := true, false
	base := Defaults("")
	base.merge(Config{Chat: Chat{Matrix: Matrix{Enabled: &on, SelfSign: &on}}})
	if !DerefBool(base.Chat.Matrix.Enabled) || !DerefBool(base.Chat.Matrix.SelfSign) {
		t.Fatal("a layer failed to turn chat and self_sign on")
	}
	base.merge(Config{Chat: Chat{Matrix: Matrix{SelfSign: &off}}})
	if DerefBool(base.Chat.Matrix.SelfSign) {
		t.Error("a layer failed to turn self_sign back off")
	}
	if !DerefBool(base.Chat.Matrix.Enabled) {
		t.Error("turning self_sign off also turned chat off")
	}
}

// TestMaxAgeIsAParseableDuration is the one chat setting with a second parser
// behind it. Storing it as a string means a typo survives Load and fails in the
// daemon instead, so the contract the daemon relies on is pinned here, along
// with the empty default that keeps every message.
func TestMaxAgeIsAParseableDuration(t *testing.T) {
	if got := Defaults("").Chat.Matrix.MaxAge; got != "" {
		t.Fatalf("default max_age = %q, want empty so a fresh install drops nothing", got)
	}
	path := filepath.Join(t.TempDir(), ".bulle.yml")
	if err := os.WriteFile(path, []byte(chatConfig), 0o644); err != nil {
		t.Fatalf("writing tmp config: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	age, err := time.ParseDuration(loaded.Chat.Matrix.MaxAge)
	if err != nil || age != 2*time.Hour {
		t.Fatalf("max_age = %q parsed to %s (err %v), want 2h", loaded.Chat.Matrix.MaxAge, age, err)
	}
}
