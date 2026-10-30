package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestCollectKeepsEveryFailure(t *testing.T) {
	errs := collect([]string{"a", "b", "c"}, func(key string) error {
		if key == "b" {
			return nil
		}
		return errors.New("boom " + key)
	})
	if len(errs) != 2 {
		t.Fatalf("errs = %v", errs)
	}
	notice := failureNotice("Retour impossible", errs)
	if !strings.Contains(notice, "a: boom a") || !strings.Contains(notice, "c: boom c") {
		t.Fatalf("notice = %q", notice)
	}
	if failureNotice("x", nil) != "" {
		t.Fatal("no error must give no notice")
	}
}

func TestAudioReplyKeepsTranscriptOnFailure(t *testing.T) {
	got := audioReply("bonjour", "", errors.New("model\ndown"))
	if !strings.Contains(got, "bonjour") || !strings.Contains(got, "La réponse a échoué : model down") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(audioReply("bonjour", "salut", nil), "échoué") {
		t.Fatal("success must not report a failure")
	}
}
