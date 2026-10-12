package main

import "time"

// durationUntilNextInterval returns the wait until the next UTC clock boundary
// aligned on interval (e.g. 5m → :00, :05, :10, …).
func durationUntilNextInterval(now time.Time, interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = time.Minute
	}
	now = now.UTC()
	next := now.Truncate(interval).Add(interval)
	return next.Sub(now)
}
