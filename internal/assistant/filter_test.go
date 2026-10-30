package assistant

import "testing"

func TestIsVIP(t *testing.T) {
	vip := []string{" Boss@Example.org "}
	cases := map[string]bool{
		"boss@example.org":  true,
		"BOSS@EXAMPLE.ORG":  true,
		"other@example.org": false,
		"":                  false,
	}
	for addr, want := range cases {
		if got := IsVIP(addr, vip); got != want {
			t.Errorf("%q: got %v, want %v", addr, got, want)
		}
	}
}

func TestClassifySender(t *testing.T) {
	drop := []string{"@Spammy.example", "boss@acme.fr"}
	cases := []struct {
		addr   string
		class  Class
		reason string
	}{
		{"noreply@x.test", ClassBulk, "noreply-sender"},
		{"hello@notify.railway.app", ClassBulk, "noreply-sender"},
		{"News@x.test", ClassBulk, "noreply-sender"},
		{"mailer-daemon@x.test", ClassBulk, "noreply-sender"},
		{"bob@spammy.example", ClassBulk, "drop-sender"},
		{"alice@example.org", ClassUnknownHuman, "no-signal"},
		{"alice@acme-marketing.fr", ClassUnknownHuman, "no-signal"},
		{"bob@denotify.io", ClassUnknownHuman, "no-signal"},
		{"newsletter@x.test", ClassBulk, "noreply-sender"},
		{"x@mail.spammy.example", ClassUnknownHuman, "no-signal"},
		{"boss@acme.fr", ClassBulk, "drop-sender"},
		{"theboss@acme.fr", ClassUnknownHuman, "no-signal"},
		{"alice@acme-marketing.fr", ClassUnknownHuman, "no-signal"},
		{"bob@denotify.io", ClassUnknownHuman, "no-signal"},
		{"newsletter@x.test", ClassBulk, "noreply-sender"},
		{"x@mail.spammy.example", ClassUnknownHuman, "no-signal"},
		{"boss@acme.fr", ClassBulk, "drop-sender"},
		{"theboss@acme.fr", ClassUnknownHuman, "no-signal"},
	}
	for _, c := range cases {
		class, reason := ClassifySender(c.addr, drop)
		if class != c.class || reason != c.reason {
			t.Errorf("%s: got %s/%s, want %s/%s", c.addr, class, reason, c.class, c.reason)
		}
	}
}
