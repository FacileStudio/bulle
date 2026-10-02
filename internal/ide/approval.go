package ide

import "context"

// Answer is what the attached editors said about one pending tool call. It is
// three-valued on purpose: "there was nobody to ask" is not the same as "the
// call was refused", and a caller with an approval surface of its own — bulle's
// terminal prompt — has to be able to tell them apart. Collapsed into a bool,
// a session that opens a socket no editor has attached to denies every call the
// moment it is asked, terminal prompt and all.
type Answer int

const (
	// Unanswered means no editor was attached, so the question was never
	// asked. The caller's own surface decides.
	Unanswered Answer = iota
	// Refused means an editor was asked and did not allow the call: it
	// answered no, or it went quiet until the caller's deadline, or its
	// connection dropped, or the session ended first.
	Refused
	// Allowed means an editor explicitly allowed this one call.
	Allowed
)

// Approve asks the attached editors to answer one pending tool call and waits
// for the answer. It fails closed: a timeout, a lost connection, or an end to
// the session is a refusal, never an implicit yes.
//
// A nil server, and a session with no editor attached, answer Unanswered rather
// than Refused — nothing was asked, so nothing was refused. That is what lets
// a session install these hooks and keep them: the no-editor path costs a nil
// check and leaves the decision where it already was.
func (s *Server) Approve(ctx context.Context, id, tool, input string) Answer {
	if s == nil {
		return Unanswered
	}
	answer, asked := s.await(id)
	if !asked {
		return Unanswered
	}
	defer s.forget(id)
	s.Publish(Approval(id, tool, input))
	select {
	case allow := <-answer:
		if allow {
			return Allowed
		}
		return Refused
	case <-ctx.Done():
		return Refused
	case <-s.done:
		return Refused
	}
}

// await registers the channel one call's answer will land on, and reports
// whether there was an editor attached to answer it. Attachment is read under
// the same lock that registers, so an editor cannot slip away between the two:
// a call registered after the last editor left would be a question nobody was
// asked, counted as a refusal.
func (s *Server) await(id string) (chan bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.conns) == 0 {
		return nil, false
	}
	answer := make(chan bool, 1)
	s.waiting[id] = answer
	return answer, true
}

// forget drops one call that is no longer waiting, and takes whatever answer
// arrived after it stopped waiting with it.
func (s *Server) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.waiting, id)
}

// answer delivers one editor's reply to the call waiting for it. An id nobody
// is waiting on is ignored rather than refused: a client that clicked late
// must not break the session it is attached to. An approve that names nothing
// answers nothing, so a reply that lost its id can never be read as a yes.
func (s *Server) answer(cmd Command) {
	if cmd.ID == "" {
		return
	}
	s.mu.Lock()
	answer, waiting := s.waiting[cmd.ID]
	s.mu.Unlock()
	if !waiting {
		return
	}
	select {
	case answer <- cmd.Allow:
	default:
	}
}
