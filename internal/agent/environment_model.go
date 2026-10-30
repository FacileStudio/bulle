package agent

import "fmt"

// modelNote tells the model which model and backend answer this session.
// Without it a model guesses from whatever environment variables it can read,
// and those describe the shell that started the process, not the session.
func modelNote(config Config) string {
	if config.Model == "" {
		return ""
	}
	return fmt.Sprintf("\n\nYou are running as the model %q through the %q backend. "+
		"Environment variables such as OPENAI_BASE_URL or GRAFT_MODEL do not describe this session; "+
		"if asked which model you are, answer with this one.", config.Model, config.Backend)
}
