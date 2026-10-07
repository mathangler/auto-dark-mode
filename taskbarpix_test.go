package main

import (
	"os"
	"testing"
)

// TestSampleTaskbarLuma exercises the A-segment sampling pipeline against the
// real desktop: window enumeration, GetWindowRect, the screen DC and GetPixel.
// It runs on the developer machine (a taskbar is always present in an
// interactive session), so unavailability means the pipeline itself is
// broken, not an environment quirk. Numeric expectations are sanity bounds
// plus consistency between the raw samples and the gate decisions — the
// absolute classification intentionally is NOT asserted, because the current
// theme at run time is whatever it is (and visual-vs-registry mismatch is
// precisely the bug this machine can exhibit).
func TestSampleTaskbarLuma(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("developer-machine test: needs an interactive desktop session with a taskbar")
	}
	med, detail, ok := sampleTaskbarLuma()
	if !ok {
		t.Fatalf("taskbar sampling failed on an interactive session: %s", detail)
	}
	if med < 0 || med > 255 {
		t.Fatalf("median luminance out of range: %v (%s)", med, detail)
	}
	t.Logf("median=%.0f samples=[%s]", med, detail)

	looksDark := med < 128

	// Gate decisions must be self-consistent with the sampled median:
	//   - a confidently-dark reading must never demand a flip to dark,
	//   - a confidently-light reading must never demand a flip to light,
	//   - ambiguous readings (111..169) must ALWAYS flip (fall back to the
	//     old blind behavior — unknown state never means "do nothing").
	needsDark, detailDark := taskbarNeedsFlip(true)
	needsLight, detailLight := taskbarNeedsFlip(false)
	t.Logf("target=dark  -> needsFlip=%v (%s)", needsDark, detailDark)
	t.Logf("target=light -> needsFlip=%v (%s)", needsLight, detailLight)

	switch {
	case med <= pixelConfidentDark:
		if needsDark {
			t.Errorf("median %.0f is confidently dark but the gate still wants a flip to dark", med)
		}
		if !needsLight {
			t.Errorf("median %.0f is dark yet the gate skips the flip to light", med)
		}
	case med >= pixelConfidentLight:
		if needsLight {
			t.Errorf("median %.0f is confidently light but the gate still wants a flip to light", med)
		}
		if !needsDark {
			t.Errorf("median %.0f is light yet the gate skips the flip to dark", med)
		}
	default:
		// Ambiguous band: the gate must flip either way.
		if !needsDark || !needsLight {
			t.Errorf("ambiguous median %.0f must fall back to flipping for both targets (dark=%v light=%v)",
				med, needsDark, needsLight)
		}
	}

	// The two decisions only ever differ in the confident bands.
	if needsDark == needsLight && (med <= pixelConfidentDark || med >= pixelConfidentLight) {
		t.Errorf("confident reading %.0f should produce opposite decisions, got dark=%v light=%v",
			med, needsDark, needsLight)
	}
	_ = looksDark
}
