package main

import (
	"context"
	"log"
	"time"
)

// runJob ticks immediately, then on every UTC clock boundary aligned to interval.
// Add more scheduled work by launching another go runJob(...) from main.
func runJob(ctx context.Context, name string, interval time.Duration, tick func(context.Context)) {
	if interval <= 0 {
		interval = time.Minute
	}

	log.Printf("scheduler: %s started (interval=%s clock-aligned)", name, interval)
	tick(ctx)

	for {
		wait := durationUntilNextInterval(time.Now(), interval)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Printf("scheduler: %s stopped", name)
			return
		case <-timer.C:
			tick(ctx)
		}
	}
}
