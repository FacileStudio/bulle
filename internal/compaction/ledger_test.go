package compaction

import (
	"strings"
	"testing"

	"github.com/FacileStudio/nacelle"
)

func TestIsLedgerRecognizesTheSentinel(t *testing.T) {
	ledger := BuildLedger("", "Decisions:\n- ship it")
	if !IsLedger(ledger) {
		t.Fatalf("IsLedger(BuildLedger(...)) = false, want true")
	}
	if IsLedger(nacelle.UserText("[compacted context] the old marker")) {
		t.Error("IsLedger matched the retired compacted-context marker")
	}
	if IsLedger(nacelle.AssistantText("an ordinary turn")) {
		t.Error("IsLedger matched an ordinary turn")
	}
}

// The sentinel grants the strongest identity this package hands out: a ledger is
// folded into and never pruned, so whatever matches it stops being history. A
// prefix match is cheap to hit by accident — a turn quoting the marker, or a
// paste that opens with it and sits ahead of the real ledger, whose first match
// wins — and the message that claimed the identity would take the real ledger's
// place while the real one read as a prune candidate. Only the whole first line
// counts.
func TestIsLedgerRequiresTheSentinelOnItsOwnLine(t *testing.T) {
	for _, text := range []string{
		Sentinel + " and here is what I copied out of the docs",
		Sentinel + " is the marker bulle writes",
		"the ledger opens with " + Sentinel + ", then the facts",
	} {
		msg := nacelle.UserText(text)
		if IsLedger(msg) {
			t.Errorf("IsLedger(%q) = true, want a text that only opens with the marker refused", text)
		}
		if body := Body(msg); body != "" {
			t.Errorf("Body(%q) = %q, want the marker left in a text that is not the ledger", text, body)
		}
	}

	for _, tc := range []struct {
		name string
		text string
		body string
	}{
		{"the sentinel alone", Sentinel, ""},
		{"the sentinel and a body", Sentinel + "\n\nDecisions:\n- ship it", "Decisions:\n- ship it"},
		{"the sentinel with room around it", "  " + Sentinel + "  \n\nbody", "body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := nacelle.AssistantText(tc.text)
			if !IsLedger(msg) {
				t.Fatalf("IsLedger(%q) = false, want the sentinel line recognised", tc.text)
			}
			if body := Body(msg); body != tc.body {
				t.Errorf("Body(%q) = %q, want %q", tc.text, body, tc.body)
			}
		})
	}
}

// typedSentinel is the marker as a reader meets it: the words a document prints
// and a person types, with the codepoint the package writes left off, because no
// keyboard produces one.
const typedSentinel = "[state ledger]"

// The marker has to be one a reader cannot produce. Identity is the strongest
// claim this package hands out: a ledger is folded into and never pruned. The
// text it is read from is text a reader supplies, so a turn that spells the
// marker the way a document prints it has to read as the ordinary history it is.
func TestIsLedgerRefusesTheMarkerAsAReaderTypesIt(t *testing.T) {
	pasted := nacelle.UserText(typedSentinel + "\n\n- a constraint I copied out of the handoff")
	if IsLedger(pasted) {
		t.Errorf("IsLedger(%q…) = true, want a turn that only types the marker left as history", typedSentinel)
	}
	if body := Body(pasted); body != "" {
		t.Errorf("Body(...) = %q, want the marker left in the turn that carries it", body)
	}
	if !IsLedger(BuildLedger("", "Decisions:\n- ship it")) {
		t.Error("IsLedger(BuildLedger(...)) = false, want the marker this package writes recognised")
	}
}

// The zone follows the marker the package writes and not the first turn that
// merely types it. `ledgerIndex` takes the first match in the anchor..active
// range, so a paste sitting ahead of the real ledger would take the ledger's zone
// if it matched, and the ledger behind it would read as history a prune may drop
// (I3a).
func TestALedgerOutranksAPasteAheadOfIt(t *testing.T) {
	conv := []nacelle.Message{
		nacelle.UserText("the question"),
		nacelle.UserText(typedSentinel + "\n\n- copied out of a document"),
		BuildLedger("", "Decisions:\n- the facts"),
		nacelle.UserText("a follow-up"),
	}
	plan := Plan(conv, Policy{AnchorMessages: 1, KeepTurns: 1})

	ledgers := Section(conv, plan, ZoneLedger)
	if len(ledgers) != 1 {
		t.Fatalf("ledger zone holds %d messages, want the one the package wrote", len(ledgers))
	}
	if body := Body(ledgers[0]); body != "Decisions:\n- the facts" {
		t.Errorf("ledger body = %q, want the ledger the package wrote claimed as the zone", body)
	}
	if !IsLedger(ledgers[0]) {
		t.Error("IsLedger(ledger zone) = false, want the package's own ledger recognised")
	}
}

// The identity is not bound to a role, and it cannot be: assembly picks the
// ledger's role as the opposite of the anchor's last turn, so a session anchored
// on an assistant turn is folded into a user-role ledger — and a later pass that
// failed to recognise it would hand the ledger to the judge as history and prune
// it. Whatever the package writes has to stay recognisable.
func TestIsLedgerRecognizesEveryLedgerThePackageWrites(t *testing.T) {
	if !IsLedger(BuildLedger("", "Decisions:\n- ship it")) {
		t.Error("IsLedger(BuildLedger(...)) = false, want the assistant-role ledger recognised")
	}

	conv := []nacelle.Message{
		nacelle.AssistantText("a session resumed on an assistant turn"),
		callMessage("c1", "read"),
		resultMessage("c1", "read", strings.Repeat("x", 40_000)),
		nacelle.AssistantText("answer one"),
		nacelle.UserText("follow up"),
		nacelle.AssistantText("answer two"),
	}
	policy := Policy{AnchorMessages: 1, KeepTurns: 2}

	out, stats := Apply(conv, Plan(conv, policy), "Decisions:\n- folded", nil, false)

	if stats.Refused || stats.Stale {
		t.Fatalf("stats = %+v, want the pass to land", stats)
	}
	if len(out) < 2 || out[1].Role != nacelle.RoleUser {
		t.Fatalf("ledger = %+v, want the user-role ledger an assistant anchor produces", out)
	}
	if !IsLedger(out[1]) {
		t.Fatalf("IsLedger(%+v) = false, want the ledger the package just wrote recognised", out[1])
	}
	if body := Body(out[1]); !strings.Contains(body, "folded") {
		t.Errorf("body = %q, want the pass's own summary read back", body)
	}
}

// A ledger built from an earlier one always contains it: a later pass can fold
// new facts in but can never quietly drop an older one.
func TestBuildLedgerFoldsMonotonically(t *testing.T) {
	first := Body(BuildLedger("", "decided A"))
	second := Body(BuildLedger(first, "learned B"))
	if !strings.Contains(second, "decided A") {
		t.Errorf("ledger = %q, want the earlier fact preserved", second)
	}
	if !strings.Contains(second, "learned B") {
		t.Errorf("ledger = %q, want the new fact folded in", second)
	}
}

// Folding a ledger into itself is a no-op, so a pass with nothing new cannot
// duplicate the content it already carries.
func TestBuildLedgerIsIdempotent(t *testing.T) {
	once := Body(BuildLedger("", "decided A\nlearned B"))
	if twice := Body(BuildLedger(once, once)); twice != once {
		t.Errorf("folding a ledger into itself changed it:\n%q\n->\n%q", once, twice)
	}
	if summary := Body(BuildLedger(once, "learned B")); summary != once {
		t.Errorf("refolding a subset changed the ledger:\n%q\n->\n%q", once, summary)
	}
}

func TestBuildLedgerCarriesTheSentinelAndBody(t *testing.T) {
	block := BuildLedger("older", "newer")
	text, ok := block.Parts[0].(nacelle.Text)
	if !ok || !strings.HasPrefix(text.Text, Sentinel) {
		t.Fatalf("ledger = %q, want the sentinel prefix", text.Text)
	}
	if !strings.Contains(text.Text, "newer") || !strings.Contains(text.Text, "older") {
		t.Errorf("ledger = %q, want both halves folded in", text.Text)
	}
}
