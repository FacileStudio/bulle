package chat

import (
	"encoding/json"
	"strings"

	"maunium.net/go/mautrix/event"
)

// audioOf returns the media of an audio message, or false. It accepts m.audio,
// and m.file whose mimetype is audio/*. For an encrypted room the file info
// (key, iv, hashes) is kept as JSON in Media.Key, and the URL comes from it.
func audioOf(content *event.MessageEventContent) (*Media, bool) {
	mime := ""
	if content.Info != nil {
		mime = content.Info.MimeType
	}
	audio := content.MsgType == event.MsgAudio ||
		(content.MsgType == event.MsgFile && strings.HasPrefix(mime, "audio/"))
	if !audio || isEdit(content.RelatesTo) {
		return nil, false
	}
	media := &Media{URL: string(content.URL), MimeType: mime}
	if content.Info != nil {
		media.Size = int64(content.Info.Size)
	}
	if content.File != nil {
		media.URL = string(content.File.URL)
		if raw, err := json.Marshal(content.File); err == nil {
			media.Key = string(raw)
		}
	}
	if media.URL == "" {
		return nil, false
	}
	return media, true
}
