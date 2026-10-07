package main

import "testing"

// A single bad fix must not silently move the sunrise/sunset boundary: the
// guard flags anything beyond locationJumpLimit degrees, while ordinary GPS
// jitter and a genuine long-distance move behave as designed.
func TestCoordJumpDegrees(t *testing.T) {
	base := LocationResult{Lat: 40.71280, Lon: -74.00600} // New York

	// normal jitter: well under the confirm threshold
	near := LocationResult{Lat: 40.71300, Lon: -74.00700}
	if d := coordJumpDegrees(base, near); d > 0.05 {
		t.Errorf("jitter should be tiny, got %.4f deg", d)
	}

	// a bad fix ~10 degrees west (the real-world failure mode) must exceed
	// the rejection limit
	bad := LocationResult{Lat: 40.72000, Lon: -84.00000}
	if d := coordJumpDegrees(base, bad); d <= locationJumpLimit {
		t.Errorf("bad fix must exceed %.1f deg, got %.4f", locationJumpLimit, d)
	}

	// a plausible travel jump must also exceed it, so the two-consecutive-fix
	// confirmation path can accept it later
	london := LocationResult{Lat: 51.50740, Lon: -0.12780}
	if d := coordJumpDegrees(base, london); d <= locationJumpLimit {
		t.Errorf("travel jump must exceed %.1f deg, got %.4f", locationJumpLimit, d)
	}
}
