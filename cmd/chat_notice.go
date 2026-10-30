package cmd

import (
	"strings"
	"unicode"
)

var frenchWords = map[string]bool{
	"le": true, "la": true, "les": true, "un": true, "une": true, "des": true,
	"est": true, "et": true, "je": true, "tu": true, "vous": true, "nous": true,
	"pas": true, "pour": true, "que": true, "quoi": true, "bonjour": true,
	"merci": true, "salut": true, "oui": true, "non": true, "mon": true,
	"peux": true, "dans": true, "avec": true,
}

func looksFrench(text string) bool {
	lower := strings.ToLower(text)
	if strings.ContainsAny(lower, "àâäçéèêëîïôöùûüÿœ") {
		return true
	}
	words := strings.FieldsFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) })
	for _, word := range words {
		if frenchWords[word] {
			return true
		}
	}
	return false
}

func emptyReplyNotice(userText string) string {
	if looksFrench(userText) {
		return "Je n'ai rien à ajouter."
	}
	return "Nothing to add."
}
