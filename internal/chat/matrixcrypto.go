package chat

import (
	"context"
	"errors"
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
)

// watchUndecryptable makes an encrypted room that will not decrypt say so, in
// words an operator can act on.
//
// Without it an unreadable message is a log line from a library nobody reads
// and a bot that appears to ignore the room. The two causes need opposite
// responses and neither is a bug in bulle: a sender whose client refuses to
// share keys with an unverified device is a policy decision the person can
// change in their own client, while a missing session is what a broken device
// looks like. Saying which one happened is the whole point.
func watchUndecryptable(helper *cryptohelper.CryptoHelper, client *mautrix.Client) error {
	helper.DecryptErrorCallback = func(evt *event.Event, err error) {
		report("matrix", fmt.Errorf("cannot read %s in %s: %w", evt.ID, evt.RoomID, err))
	}
	syncer, ok := client.Syncer.(mautrix.ExtensibleSyncer)
	if !ok {
		return errors.New("matrix: client syncer cannot carry a withheld-key handler")
	}
	syncer.OnEventType(event.ToDeviceRoomKeyWithheld, func(_ context.Context, evt *event.Event) {
		withheld := evt.Content.AsRoomKeyWithheld()
		report("matrix", fmt.Errorf("keys withheld for room %s: %s (%s)",
			withheld.RoomID, withheld.Code, withheld.Reason))
	})
	return nil
}

// selfSign makes the bot's device a signed part of the account's cross-signing
// identity, which is what keeps the room readable once senders stop sharing
// keys with devices that are not cross-signed.
//
// It matters because the alternative is a human tapping emoji in Element on a
// device that only exists to answer messages, and interactive verification is
// the one thing a headless bot cannot do.
//
// Three cases, and the third is the one that needs a human:
//
//   - no identity yet: generate one, sign this device, and hand back the
//     recovery key, which is the only way to restore those keys onto a fresh
//     database since they otherwise live in the account's server-side SSSS.
//     A homeserver does not demand user-interactive auth to publish a first
//     master key, which is why this works unattended.
//   - an identity, with this device already signed by it: nothing to do.
//   - an identity bulle cannot sign with, which is what logging Element into the
//     bot's account first produces. Nothing here can fix that: replacing the
//     identity needs user-interactive auth, and a homeserver that has moved to
//     OAuth offers no password stage at all (matrix.org offers only m.oauth and
//     a browser URL). The remedy is the operator's, so the error says so.
func selfSign(ctx context.Context, helper *cryptohelper.CryptoHelper, password, recoveryKey string) (string, error) {
	mach := helper.Machine()
	hasKeys, verified, err := mach.GetOwnVerificationStatus(ctx)
	if err != nil {
		return "", fmt.Errorf("matrix: reading verification status: %w", err)
	}
	switch {
	case !hasKeys:
		return generateCrossSigning(ctx, helper, password)
	case verified:
		report("matrix", errors.New("cross-signing is in place and this device is already signed by it"))
		return "", nil
	default:
		if recoveryKey != "" {
			if err := mach.VerifyWithRecoveryKey(ctx, recoveryKey); err != nil {
				return "", fmt.Errorf("matrix: verifying with recovery key: %w", err)
			}
			report("matrix", errors.New("cross-signing verified with recovery key"))
			return "", nil
		}
		return "", errors.New("matrix: account has existing cross-signing keys that bulle cannot sign without credentials; set recovery_key in config or run bulle chat verify")
	}
}

// generateCrossSigning publishes the keys through user-interactive auth. The
// callback is only needed by homeservers that demand a password to publish
// keys, which Synapse does for anything past the first master key, so a bot
// configured with a token and no password can fail here and say why.
func generateCrossSigning(ctx context.Context, helper *cryptohelper.CryptoHelper, password string) (string, error) {
	mach := helper.Machine()
	if password == "" {
		key, err := mach.GenerateAndVerifyWithRecoveryKey(ctx)
		if err != nil {
			return "", fmt.Errorf("matrix: self-signing: %w (a homeserver demanding user-interactive auth needs password or password_command set)", err)
		}
		return key, nil
	}
	key, _, err := mach.GenerateAndUploadCrossSigningKeysWithPassword(ctx, password, "")
	if err != nil {
		return "", fmt.Errorf("matrix: self-signing: %w", err)
	}
	if err := mach.SignOwnDevice(ctx, mach.OwnIdentity()); err != nil {
		return "", fmt.Errorf("matrix: signing own device: %w", err)
	}
	if err := mach.SignOwnMasterKey(ctx); err != nil {
		return "", fmt.Errorf("matrix: signing own master key: %w", err)
	}
	return key, nil
}

// RecoveryKey is the cross-signing recovery key generated on the run that first
// created the bot's identity, and empty on every run after. The caller stores
// it once: it is the only way to recover the signing keys onto a fresh
// database, since they otherwise live only in the account's server-side SSSS.
func (m *Matrix) RecoveryKey() string { return m.recovery }

// watchTombstone reports a room that was replaced by an upgrade. It cannot// re-key on its own and must not try: an upgrade changes the room ID, so the
// old conversation is over and the new room is a different one that nobody has
// said may reach a shell. What it can do is refuse to be silent about it, since
// the failure without this line is a bot that keeps running and simply stops
// answering, with an allowlist that still names a dead room.
func (m *Matrix) watchTombstone() error {
	syncer, ok := m.client.Syncer.(mautrix.ExtensibleSyncer)
	if !ok {
		return errors.New("matrix: client syncer cannot carry a tombstone handler")
	}
	syncer.OnEventType(event.StateTombstone, func(_ context.Context, evt *event.Event) {
		replacement := evt.Content.AsTombstone().GetReplacementRoom()
		report("matrix", fmt.Errorf(
			"room %s was upgraded to %s: this conversation is over, add the new room to chat.matrix.rooms if it should reach bulle",
			evt.RoomID, replacement))
	})
	return nil
}
