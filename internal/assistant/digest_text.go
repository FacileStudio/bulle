package assistant

import (
	"fmt"
	"strings"
)

const debriefInvite = "Comment s'est passée ta journée ? Si tu veux faire ton débrief, je suis là."

var digestTitles = map[string]string{
	"morning": "Ton point du matin :",
	"midday":  "Ton point de mi-journée :",
	"evening": "Ton point du soir :",
}

var digestTitlesEN = map[string]string{
	"morning": "Your morning brief:",
	"midday":  "Your midday brief:",
	"evening": "Your evening brief:",
}

func templateDigest(p Payload, en bool) string {
	titles := digestTitles
	if en {
		titles = digestTitlesEN
	}
	lines := []string{titles[p.Digest]}
	for _, it := range p.Items {
		lines = append(lines, "- "+templateLine(it, en))
	}
	if len(p.Unknown) > 0 {
		lines = append(lines, "- "+unknownLine(p.Unknown, en))
	}
	return strings.Join(lines, "\n")
}

func templateLine(it DigestItem, en bool) string {
	labels := map[string]string{
		"event": "%s : %s", "overdue": "En retard : %[2]s",
		"due_today": "Pour aujourd'hui : %[2]s", "mail": "Mail de %[3]s : %[2]s",
	}
	if en {
		labels = map[string]string{
			"event": "%s: %s", "overdue": "Overdue: %[2]s",
			"due_today": "Due today: %[2]s", "mail": "Mail from %[3]s: %[2]s",
		}
	}
	return fmt.Sprintf(labels[it.Kind], it.At, it.Title, it.Who)
}

func unknownLine(names []string, en bool) string {
	if en {
		return "Messages from people I do not know yet: " + strings.Join(names, ", ")
	}
	return "Messages de personnes que je ne connais pas encore : " + strings.Join(names, ", ")
}

func withDebrief(text string) string {
	if strings.Contains(text, debriefInvite) {
		return text
	}
	return text + "\n" + debriefInvite
}
