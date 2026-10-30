package memory

import (
	"regexp"
	"strings"
)

var (
	keywordSecret = regexp.MustCompile(`(?i)\b(?:password|passwd|mot de passe|mdp|token|secret|api[ _-]?key|cl[ée] api|private key)\b\s*(?:(?:is|est)\b|[:=])`)
	safePointer   = regexp.MustCompile(`(?i)^\s*(?:dans|in|on|sur|chez|via|stored|stock[ée]e?)\b`)
	longToken     = regexp.MustCompile(`[A-Za-z0-9_=+-]{32,}`)
	commitSHA     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	knownSecrets  = []*regexp.Regexp{
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
		regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),
	}
)

func looksSecret(fact string) bool {
	for _, p := range knownSecrets {
		if p.MatchString(fact) {
			return true
		}
	}
	for _, loc := range keywordSecret.FindAllStringIndex(fact, -1) {
		if !safePointer.MatchString(fact[loc[1]:]) {
			return true
		}
	}
	return hasOpaqueToken(fact)
}

func hasOpaqueToken(fact string) bool {
	for _, tok := range longToken.FindAllString(fact, -1) {
		if commitSHA.MatchString(tok) || strings.Trim(tok, "-_=+") == "" {
			continue
		}
		if strings.ContainsAny(tok, "0123456789") && strings.ContainsAny(strings.ToLower(tok), "abcdefghijklmnopqrstuvwxyz") {
			return true
		}
	}
	return false
}
