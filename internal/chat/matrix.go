package chat

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// deviceName is what the bot's device is called on the homeserver, and the
// name a human sees in Element's device list.
const deviceName = "bulle"

// Matrix is one bot account's connection to a homeserver and the whole of the
// platform-specific half of this package: it owns the /sync loop and the
// encrypted send path and nothing else. Authorization and session keying
// happen above it, so a second platform arrives with no second opinion about
// who may make this machine run a tool.
//
// The crypto state lives in the database handed to OpenStore, never in this
// struct, so a restart resumes the same device rather than registering a new
// one and re-reading the whole backlog.
type Matrix struct {
	client   *mautrix.Client
	helper   *cryptohelper.CryptoHelper
	self     id.UserID
	recovery string
}

var _ Adapter = (*Matrix)(nil)
var _ Typer = (*Matrix)(nil)

// MatrixConfig is everything one bot needs to come up. Token and Password are
// alternatives, not partners: a token is used as given, while a password logs
// in and is only consulted when the token is empty. The device ID matters only
// on the token path, when the database has no device to resume.
type MatrixConfig struct {
	Homeserver  string
	UserID      string
	DeviceID    string
	Token       string
	Password    string
	PickleKey   []byte
	Database    *dbutil.Database
	SelfSign    bool
	RecoveryKey string
}

// NewMatrix logs in, loads the crypto device and returns an adapter ready to
// sync. It takes no context because the constructor is a frozen seam that
// cannot carry one: the login and the store upgrade it performs are bounded by
// the homeserver's own request timeout.
func NewMatrix(cfg MatrixConfig) (*Matrix, error) {
	client, err := newMatrixClient(cfg)
	if err != nil {
		return nil, err
	}
	helper, err := newCryptoHelper(client, cfg)
	if err != nil {
		return nil, err
	}
	if err := helper.Init(context.Background()); err != nil {
		return nil, fmt.Errorf("matrix: starting crypto for %s: %w", cfg.UserID, err)
	}
	client.Crypto = helper
	if err := watchUndecryptable(helper, client); err != nil {
		return nil, err
	}
	recovery, err := startSelfSign(cfg, helper)
	if err != nil {
		report("matrix", fmt.Errorf("cross-signing did not complete, carrying on without it: %w", err))
	}
	return &Matrix{client: client, helper: helper, self: client.UserID, recovery: recovery}, nil
}

// startSelfSign runs the cross-signing step when the configuration asked for
// it, and stays out of the way otherwise. A bot whose operator signs the device
// by hand must not have keys published behind their back: this writes account
// data on the homeserver.
//
// Its failure is never fatal, which the caller enforces by reporting rather
// than returning. Chat works without cross-signing: the device is simply not
// signed, so it decrypts only while senders share keys with such devices. A
// homeserver that refuses to publish keys, or is briefly unreachable, must not
// take the whole daemon down with it, and a unit configured Restart=on-failure
// would otherwise turn a permanent refusal into a restart loop.
func startSelfSign(cfg MatrixConfig, helper *cryptohelper.CryptoHelper) (string, error) {
	if !cfg.SelfSign {
		return "", nil
	}
	return selfSign(context.Background(), helper, cfg.Password, cfg.RecoveryKey)
}

// newMatrixClient builds the plain HTTP client. The logger is set here because
// mautrix leaves it at zerolog.Nop(), which would turn a homeserver refusing
// every request into a sync loop that fails in silence.
func newMatrixClient(cfg MatrixConfig) (*mautrix.Client, error) {
	if cfg.Homeserver == "" || cfg.UserID == "" {
		return nil, errors.New("matrix: homeserver and user_id are required")
	}
	client, err := mautrix.NewClient(cfg.Homeserver, id.UserID(cfg.UserID), cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("matrix: opening %s: %w", cfg.Homeserver, err)
	}
	if cfg.DeviceID != "" {
		client.DeviceID = id.DeviceID(cfg.DeviceID)
	}
	client.Log = zerolog.New(os.Stderr).Level(zerolog.WarnLevel).With().Timestamp().Logger()
	return client, nil
}

// newCryptoHelper builds the helper that owns both stores. The database is
// required rather than optional: without it every restart would create a new
// device, which both loses decryption and floods the homeserver.
//
// It also picks the authentication route, and the two are alternatives rather
// than layers. A token is used exactly as given. With no token it sets LoginAs
// so the helper logs in with the password, which is also the run that creates
// the device, and so the only run where the display name can be set: the
// homeserver ignores that field once the device exists. That name matters
// because Element lists the bot beside the person's own verified devices, and
// an unnamed string of characters there is a device nobody can recognise.
//
// A password login reuses the stored device id, which costs nothing and keeps
// the bot the same device across restarts. The homeserver does treat a login
// as authoritative for that device and invalidates its previous access token,
// so a token exported for the same device stops working once the password
// route runs. That is the reason to pick one route and stay on it.
func newCryptoHelper(client *mautrix.Client, cfg MatrixConfig) (*cryptohelper.CryptoHelper, error) {
	if cfg.Database == nil {
		return nil, errors.New("matrix: no crypto database, call chat.OpenStore first")
	}
	helper, err := cryptohelper.NewCryptoHelper(client, cfg.PickleKey, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("matrix: building crypto helper: %w", err)
	}
	if cfg.Token == "" {
		helper.LoginAs = &mautrix.ReqLogin{
			Type:                     mautrix.AuthTypePassword,
			Identifier:               mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: cfg.UserID},
			Password:                 cfg.Password,
			InitialDeviceDisplayName: deviceName,
		}
	}
	return helper, nil
}

// Name identifies this adapter in every Identity it produces and prefixes the
// session key, so it must stay the string the rest of bulle already expects.
func (m *Matrix) Name() string { return "matrix" }

// Receive registers the message handler, then blocks in /sync until the
// context is cancelled, handing each accepted message to handle in arrival
// order. Only event.EventMessage is watched: the crypto helper already
// decrypts m.room.encrypted and re-dispatches the plaintext event, so watching
// the ciphertext too would deliver every message twice.
//
// A cancelled context is a clean stop, not an error: a daemon shutting down is
// not a platform failure, and the caller asked for exactly that.
func (m *Matrix) Receive(ctx context.Context, handle func(Message)) error {
	syncer, ok := m.client.Syncer.(mautrix.ExtensibleSyncer)
	if !ok {
		return errors.New("matrix: client syncer cannot carry a message handler")
	}
	syncer.OnEventType(event.EventMessage, func(_ context.Context, evt *event.Event) {
		if msg, ok := messageFrom(evt, m.self); ok {
			handle(msg)
		}
	})
	if err := m.watchInvites(); err != nil {
		return err
	}
	if err := m.watchTombstone(); err != nil {
		return err
	}
	err := m.client.SyncWithContext(ctx)
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		err = nil
	}
	return err
}

// watchInvites joins a room the bot has been invited to. Nothing in the
// library does it: invites are dispatched as membership events and never
// accepted, so a bot that skipped this is indistinguishable from one that is
// broken, and every doc for the feature has to start by telling the person to
// invite the bot.
func (m *Matrix) watchInvites() error {
	syncer, ok := m.client.Syncer.(mautrix.ExtensibleSyncer)
	if !ok {
		return errors.New("matrix: client syncer cannot carry a membership handler")
	}
	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		if evt.GetStateKey() != m.self.String() || evt.Content.AsMember().Membership != event.MembershipInvite {
			return
		}
		if _, err := m.client.JoinRoomByID(ctx, evt.RoomID); err != nil {
			report("matrix", fmt.Errorf("joining %s: %w", evt.RoomID, err))
			return
		}
		fmt.Fprintf(os.Stderr, "bulle chat: matrix: joined %s\n", evt.RoomID)
	})
	return nil
}

// Close releases the crypto database. It tolerates a helper that was never
// built, so a Matrix that failed to come up can still be closed by a caller
// that only holds the error.
func (m *Matrix) Close() error {
	if m.helper == nil {
		return nil
	}
	return m.helper.Close()
}
