package checker

import "time"

// Result holds the outcome of a single node check.
type Result struct {
	URL     string
	Latency time.Duration
	RealIP  string
	Country string
	Stage   string
	Err     error
}

// Passed reports whether the node passed all configured stages.
func (r Result) Passed() bool { return r.Err == nil }
