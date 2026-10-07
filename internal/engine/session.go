package engine

import (
	"github.com/FacileStudio/nacelle"
)

// Session coordinates multi-turn agent execution with conversation state.
type Session struct {
	conv  *Conversation
	agent *nacelle.Agent
}

// NewSession constructs a Session with the given agent and conversation.
func NewSession(agent *nacelle.Agent, conv *Conversation) *Session {
	if conv == nil {
		conv = NewConversation()
	}
	return &Session{
		conv:  conv,
		agent: agent,
	}
}

// Conversation returns the session's Conversation.
func (s *Session) Conversation() *Conversation {
	return s.conv
}

// Agent returns the session's nacelle Agent.
func (s *Session) Agent() *nacelle.Agent {
	return s.agent
}
