package sandbox

// SessionOptions configures a host-side session whose tools execute in a
// target. bulle itself runs on the host, so no binary, key or config is ever
// shipped into the target: only tool calls cross the SSH boundary.
type SessionOptions struct {
	Target      *Target
	WorkDir     string
	User        string
	Snapshot    bool
	PrintPrompt string
	SkipGuard   bool
	Runner      Runner
}
