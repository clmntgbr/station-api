package main

import (
	"testing"
	"time"
)

func TestDurationUntilNextInterval_ThirtyMinutes(t *testing.T) {
	now := time.Date(2026, 10, 12, 15, 12, 34, 0, time.UTC)
	got := durationUntilNextInterval(now, 30*time.Minute)
	want := time.Date(2026, 10, 12, 15, 30, 0, 0, time.UTC).Sub(now)
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestDurationUntilNextInterval_OnBoundary(t *testing.T) {
	now := time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC)
	got := durationUntilNextInterval(now, 30*time.Minute)
	want := 30 * time.Minute
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
