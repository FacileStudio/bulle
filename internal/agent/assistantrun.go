package agent

import "time"

// AssistantRun is one assistant run line in the central cron log.
type AssistantRun struct {
	Name        string
	Started     time.Time
	Status      string
	Surfaced    int
	ModelCalled bool
	Delivery    string
	Error       string
}

// AppendAssistantRun appends the run to cron.jsonl beside the cron job runs.
func AppendAssistantRun(r AssistantRun) error {
	called := r.ModelCalled
	return appendCronRun(cronRun{
		Name:        r.Name,
		Started:     r.Started.Format(time.RFC3339),
		DurationMs:  time.Since(r.Started).Milliseconds(),
		Status:      r.Status,
		Error:       r.Error,
		Surfaced:    &r.Surfaced,
		ModelCalled: &called,
		Delivery:    r.Delivery,
	})
}
