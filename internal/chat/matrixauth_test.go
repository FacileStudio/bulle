package chat

import (
	"context"
	"path/filepath"
	"testing"

	"go.mau.fi/util/dbutil"

	"maunium.net/go/mautrix"
)

// testStore is a crypto database in a temp directory, which is all the helper
// constructor needs to accept the managed-store path.
func testStore(t *testing.T) *dbutil.Database {
	t.Helper()
	db, err := OpenStore(filepath.Join(t.TempDir(), "matrix.db"))
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// authConfig is a credential set with everything but the auth route filled in,
// so a test can vary only the token and the password.
func authConfig(id string) MatrixConfig {
	return MatrixConfig{
		Homeserver: "https://matrix.example.org",
		UserID:     id,
		PickleKey:  []byte("0123456789abcdef0123456789abcdef"),
	}
}

func helperFor(t *testing.T, cfg MatrixConfig) *matrixAuth {
	t.Helper()
	cfg.Database = testStore(t)
	client, err := newMatrixClient(cfg)
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}
	helper, err := newCryptoHelper(client, cfg)
	if err != nil {
		t.Fatalf("building the crypto helper: %v", err)
	}
	return &matrixAuth{login: helper.LoginAs, client: client}
}

// matrixAuth is what a test reads back: the login the helper will perform, and
// the client it was built against.
type matrixAuth struct {
	login  *mautrix.ReqLogin
	client *mautrix.Client
}

// TestAPasswordWithNoTokenIsALogin is the password route: with no token the
// helper logs in itself, which is also the only run that can name the device,
// since a homeserver ignores the display name once the device exists.
func TestAPasswordWithNoTokenIsALogin(t *testing.T) {
	cases := map[string]string{
		"a full user id":   "@bulle:example.org",
		"a bare localpart": "bulle",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := authConfig(id)
			cfg.Password = "hunter2"
			auth := helperFor(t, cfg)
			if auth.login == nil {
				t.Fatal("no login: a password with no token must log in")
			}
			if auth.login.Type != mautrix.AuthTypePassword {
				t.Errorf("login type = %q, want m.login.password", auth.login.Type)
			}
			if auth.login.Identifier.User != id {
				t.Errorf("identifier user = %q, want %q passed through as given", auth.login.Identifier.User, id)
			}
			if auth.login.Password != cfg.Password {
				t.Errorf("login password = %q, want the configured one", auth.login.Password)
			}
			if auth.login.InitialDeviceDisplayName != deviceName {
				t.Errorf("display name = %q, want %q so Element names the device", auth.login.InitialDeviceDisplayName, deviceName)
			}
		})
	}
}

// TestATokenIsUsedAsGiven covers the other route: a token authenticates without
// a login, so setting LoginAs would be harmless but would also mean the helper
// never uses the credential it was handed.
func TestATokenIsUsedAsGiven(t *testing.T) {
	cfg := authConfig("@bulle:example.org")
	cfg.Token = "syt_example"
	auth := helperFor(t, cfg)
	if auth.login != nil {
		t.Error("a login was configured for a token, want the token used as given")
	}
	if auth.client.AccessToken != cfg.Token {
		t.Errorf("client token = %q, want the configured one", auth.client.AccessToken)
	}
}

// TestATokenWinsOverAPassword pins the precedence between the two routes. The
// config can carry both, and the choice is silent otherwise: a person who
// leaves a token behind after switching to a password would keep using the
// token, or the reverse, with nothing on screen to say so.
func TestATokenWinsOverAPassword(t *testing.T) {
	cfg := authConfig("@bulle:example.org")
	cfg.Token = "syt_example"
	cfg.Password = "hunter2"
	auth := helperFor(t, cfg)
	if auth.login != nil {
		t.Error("a login was configured while a token was also set, want the token to win")
	}
}

// TestTheDeviceIDIsOnlySetWhenConfigured guards the token path: DeviceID is
// optional there, and an unset one must stay unset rather than becoming the
// empty string that a login would then try to reuse.
func TestTheDeviceIDIsOnlySetWhenConfigured(t *testing.T) {
	cfg := authConfig("@bulle:example.org")
	cfg.Token = "syt_example"
	if auth := helperFor(t, cfg); auth.client.DeviceID != "" {
		t.Errorf("device id = %q, want unset", auth.client.DeviceID)
	}
	cfg.DeviceID = "JLAFKJWSCS"
	if auth := helperFor(t, cfg); auth.client.DeviceID.String() != "JLAFKJWSCS" {
		t.Errorf("device id = %q, want the configured one", auth.client.DeviceID)
	}
}

func TestVerifyWithRecoveryKeyEmptyRefused(t *testing.T) {
	m := &Matrix{}
	if err := m.VerifyWithRecoveryKey(context.Background(), ""); err == nil {
		t.Fatal("expected error on empty recovery key")
	}
}
