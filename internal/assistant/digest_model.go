package assistant

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const maxDigestChars = 1200

const digestRules = "Tu écris un message court en français, 2 à 5 lignes, ton direct, tutoiement.\n" +
	"Utilise uniquement les éléments du JSON. N'invente rien.\n" +
	"Le champ sent_recently liste des mails que l'utilisateur a envoyés : c'est du contexte pour comprendre ses échanges (ce qu'il attend, ce qu'il a promis), ne le liste pas et ne le résume pas.\n" +
	"Ne mentionne jamais le champ bulk_never_mention, ni des filtres, ni tes vérifications, ni des outils.\n" +
	"Réponds uniquement avec le texte final entre <deliver> et </deliver>.\n"

func digestPrompt(p Payload) (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return digestRules + "JSON:\n" + string(data), nil
}

func extractDeliverable(text string) string {
	const open, closing = "<deliver>", "</deliver>"
	end := strings.LastIndex(text, closing)
	if end < 0 {
		return ""
	}
	start := strings.LastIndex(text[:end], open)
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(text[start+len(open) : end])
}

func modelDigest(ctx context.Context, d Deps, p Payload) (string, bool) {
	if d.Complete == nil {
		return "", false
	}
	prompt, err := digestPrompt(p)
	if err != nil {
		return "", false
	}
	out, err := d.Complete.Complete(ctx, prompt)
	if err != nil {
		return "", true
	}
	text := extractDeliverable(out)
	if text == "" || utf8.RuneCountInString(text) > maxDigestChars {
		return "", true
	}
	return text, true
}
