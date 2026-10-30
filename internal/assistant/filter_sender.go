package assistant

import "strings"

var bulkLocalParts = map[string]bool{
	"noreply": true, "no-reply": true, "donotreply": true, "do-not-reply": true,
	"notifications": true, "notify": true, "mailer-daemon": true, "postmaster": true,
	"newsletter": true, "news": true, "marketing": true, "updates": true,
}

var bulkDomainLabels = map[string]bool{
	"notify": true, "notifications": true, "newsletter": true, "newsletters": true,
	"marketing": true, "mailer": true, "bounce": true, "bounces": true,
}

// ClassifySender is the local fallback when courrier cannot classify: built-in
// automated-sender rules and the user's drop list make a row bulk, anything
// else is an unknown human.
func ClassifySender(addr string, drop []string) (Class, string) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	if MatchesDrop(addr, drop) {
		return ClassBulk, "drop-sender"
	}
	if automatedAddress(addr) {
		return ClassBulk, "noreply-sender"
	}
	return ClassUnknownHuman, "no-signal"
}

// MatchesDrop reports whether addr equals a drop entry or ends with an
// "@domain" entry. Anything else, substrings included, never matches.
func MatchesDrop(addr string, drop []string) bool {
	addr = strings.ToLower(strings.TrimSpace(addr))
	for _, p := range drop {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if p == addr || (strings.HasPrefix(p, "@") && strings.HasSuffix(addr, p)) {
			return true
		}
	}
	return false
}

func automatedAddress(addr string) bool {
	at := strings.LastIndex(addr, "@")
	if at < 0 {
		return false
	}
	local, domain := addr[:at], addr[at+1:]
	if i := strings.Index(local, "+"); i > 0 {
		local = local[:i]
	}
	label, _, _ := strings.Cut(domain, ".")
	return bulkLocalParts[local] || bulkDomainLabels[label]
}
