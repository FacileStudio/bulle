package agent

// closeOnErr returns the original err if cleanup succeeds, or the cleanup
// error if cleanup fails. The caller should prefer the cleanup error only
// when it wants to surface close failures over the original failure.
func closeOnErr(err error, closers ...any) error {
	if err == nil {
		return nil
	}
	if cerr := closeAll(closers...); cerr != nil {
		return cerr
	}
	return err
}

// closeAll calls Close on every closer it receives. It returns the last
// error returned by a Close() error call, if any.
func closeAll(closers ...any) error {
	var lastErr error
	for _, c := range closers {
		if c == nil {
			continue
		}
		switch v := c.(type) {
		case interface{ Close() error }:
			if err := v.Close(); err != nil {
				lastErr = err
			}
		case interface{ Close() }:
			v.Close()
		}
	}
	return lastErr
}
