package memory

import (
	"errors"
	"regexp"
	"strings"
)

// ErrSecret is returned for a fact that looks like a credential.
var ErrSecret = errors.New("memory: the fact looks like a secret, store where to find it instead")

// ErrInstruction is returned for a fact that reads as an order to the agent
// rather than a fact about the user.
var ErrInstruction = errors.New("memory: the fact reads like an instruction, only store facts about the user")

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(password|passwd|mot de passe|mdp|token|secret|api[ _-]?key|cl[ée] api|private key)\b\s*(is|est|:|=)`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`[A-Za-z0-9+/_=-]{40,}`),
}

var instructionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore (all |any )?(previous|prior|above|earlier) (instructions|rules)`),
	regexp.MustCompile(`(?i)ignore (toutes )?(les )?(instructions|r[èe]gles) pr[ée]c[ée]dentes`),
	regexp.MustCompile(`(?i)\b(from now on|d[ée]sormais|dor[ée]navant),? (you|tu) (must|should|dois|devras)\b`),
	regexp.MustCompile(`(?i)\b(always|toujours) (forward|transf[ée]r|send|envoie)\b.*\b(to|[àa])\b.*@`),
}

func checkFact(fact string) error {
	for _, p := range secretPatterns {
		if p.MatchString(fact) {
			return ErrSecret
		}
	}
	for _, p := range instructionPatterns {
		if p.MatchString(fact) {
			return ErrInstruction
		}
	}
	return nil
}

func normalized(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func lineFact(line string) string {
	line = strings.TrimSpace(line)
	if rest, ok := strings.CutPrefix(line, "- ["); ok {
		if _, after, found := strings.Cut(rest, "] "); found {
			return normalized(after)
		}
	}
	return normalized(strings.TrimPrefix(line, "- "))
}

func (s *Store) known(fact string) (string, bool) {
	want := normalized(fact)
	for _, file := range []string{UserFile, MemoryFile} {
		text, _ := s.readFile(s.path(file))
		for line := range strings.SplitSeq(text, "\n") {
			if line != "" && lineFact(line) == want {
				return strings.TrimSpace(line), true
			}
		}
	}
	return "", false
}

func inNote(note, fact string) (string, bool) {
	want := normalized(fact)
	for line := range strings.SplitSeq(note, "\n") {
		if line != "" && lineFact(line) == want {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

func vetFacts(facts []string) (allowed, refused []string) {
	for _, f := range facts {
		if checkFact(f) != nil {
			refused = append(refused, f)
			continue
		}
		allowed = append(allowed, f)
	}
	return allowed, refused
}

func tail(text string) string {
	runes := []rune(text)
	if len(runes) <= maxSourceChars {
		return text
	}
	return string(runes[len(runes)-maxSourceChars:])
}
