package benchmarks

import (
	"testing"
	"time"
)

// TestRunProfileSmall exercises the small profile and asserts the
// runner returns a populated Result without error. This is the smoke
// test for the benchmarks package — it must exist so the package is
// not reported as `[no test files]` by `go test`.
func TestRunProfileSmall(t *testing.T) {
	r, err := RunProfile("small", 5)
	if err != nil {
		t.Fatalf("RunProfile(small, 5) error: %v", err)
	}
	if r.Profile != "small" {
		t.Errorf("Profile = %q, want %q", r.Profile, "small")
	}
	if r.Rounds != 5 {
		t.Errorf("Rounds = %d, want 5", r.Rounds)
	}
	if r.TotalDuration <= 0 {
		t.Errorf("TotalDuration = %v, want > 0", r.TotalDuration)
	}
	if r.AvgDuration <= 0 {
		t.Errorf("AvgDuration = %v, want > 0", r.AvgDuration)
	}
}

// TestRunProfileUnknownProfile asserts the runner rejects an unknown
// profile name with a domain error rather than panicking.
func TestRunProfileUnknownProfile(t *testing.T) {
	_, err := RunProfile("does-not-exist", 1)
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
}

// TestRunProfileMediumRounds asserts the runner completes a non-trivial
// round count on the medium profile and that the average duration is
// bounded (sanity check, not a perf gate).
func TestRunProfileMediumRounds(t *testing.T) {
	r, err := RunProfile("medium", 20)
	if err != nil {
		t.Fatalf("RunProfile(medium, 20) error: %v", err)
	}
	if r.Rounds != 20 {
		t.Errorf("Rounds = %d, want 20", r.Rounds)
	}
	// Sanity: 20 rounds on the medium profile should finish well
	// under 30s on any reasonable machine.
	if r.TotalDuration > 30*time.Second {
		t.Errorf("TotalDuration = %v, want < 30s", r.TotalDuration)
	}
}