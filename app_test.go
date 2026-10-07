package main

import (
	"testing"
	"time"
)

func TestResumeDetected(t *testing.T) {
	cases := []struct {
		name     string
		wall     time.Duration
		runtime  time.Duration
		expected bool
	}{
		{"normal 30s tick", 30 * time.Second, 30 * time.Second, false},
		{"short sleep under the 30s missing-time threshold", 25 * time.Second, 1 * time.Second, false},
		{"sleep 45 seconds", 45 * time.Second, 2 * time.Second, true},
		{"sleep 10 minutes", 10 * time.Minute, 1 * time.Second, true},
		{"sleep 2 hours", 2 * time.Hour, 2 * time.Second, true},
		{"hibernate 8 hours", 8 * time.Hour, 0, true},
		{"scheduler blocked 5 minutes, not asleep", 5 * time.Minute, 5 * time.Minute, false},
		{"wall clock jumped backwards", -90 * time.Second, 30 * time.Second, false},
		// Modern Standby with periodic maintenance: some runtime accrues
		// during the "sleep", but 30+ seconds of missing runtime still
		// reveals it.
		{"modern standby mostly idle", 2 * time.Hour, 20 * time.Minute, true},
	}
	for _, c := range cases {
		if got := resumeDetected(c.wall, c.runtime); got != c.expected {
			t.Errorf("%s: resumeDetected(%v, %v) = %v, want %v", c.name, c.wall, c.runtime, got, c.expected)
		}
	}
}
