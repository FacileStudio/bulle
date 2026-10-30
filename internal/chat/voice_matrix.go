package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"maunium.net/go/mautrix/crypto/attachment"
	"maunium.net/go/mautrix/id"
)

var _ Downloader = (*Matrix)(nil)

// Download fetches an mxc media file and decrypts it when Key carries the
// encrypted file info.
func (m *Matrix) Download(ctx context.Context, media Media) ([]byte, error) {
	uri, err := id.ParseContentURI(media.URL)
	if err != nil {
		return nil, fmt.Errorf("parse media url: %w", err)
	}
	data, err := m.client.DownloadBytes(ctx, uri)
	if err != nil {
		return nil, err
	}
	if media.Key == "" {
		return data, nil
	}
	var file attachment.EncryptedFile
	if err := json.Unmarshal([]byte(media.Key), &file); err != nil {
		return nil, fmt.Errorf("parse media key: %w", err)
	}
	if err := file.PrepareForDecryption(); err != nil {
		return nil, err
	}
	if err := file.DecryptInPlace(data); err != nil {
		return nil, err
	}
	return data, nil
}
