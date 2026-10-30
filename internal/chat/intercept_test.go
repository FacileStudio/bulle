package chat

import (
	"context"
	"testing"
)

func TestInterceptTakesTextBeforeTheQueue(t *testing.T) {
	var seen []string
	cmds := &Commands{Intercept: func(m Message) bool {
		seen = append(seen, m.Text)
		return m.Text == "oui"
	}}
	a := &fakeAdapter{in: []Message{msg("!r:x", "@alice:x", "oui"), msg("!r:x", "@alice:x", "hello")}}
	if err := RunWith(context.Background(), a, echo, Router{Allow: []string{"@alice:x"}}, cmds); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("intercept saw %v", seen)
	}
	if got := a.replies(); len(got) != 1 {
		t.Fatalf("only the second message should be answered, got %v", got)
	}
}
