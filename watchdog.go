package main

import (
	"fmt"
	"sync"
	"time"
)

// Visual watchdog (1.3.0) — closes the hole left after the 3-minute healing
// window: if the shell relapses into a stale theme AFTER the window closes,
// nothing previously could heal it (unchanged-value broadcasts are ignored)
// and the user had to click the tray manually.
//
// After every AUTOMATIC real theme transition (wake apply or scheduler tick)
// one session runs:
//   +45s / +2m   reinforce poke  (no registry touch — pure delivery)
//   +1m/+3m/+8m/+15m  pixel sample of the taskbar:
//     confidently wrong  -> flip heal (the only thing that re-syncs a stale
//                           shell cache; invisible when the bar is already
//                           showing the wrong color)
//     correct/uncertain -> reinforce poke only (a watchdog NEVER flashes on
//                          an ambiguous reading — no false positives)
//
// Honest boundary: the relapse itself is Windows-side and cannot be
// prevented with certainty; the pokes lower its odds, the samples guarantee
// any relapse is healed at the next sample point without manual action.

var (
	visualWatchMu        sync.Mutex
	visualWatchScheduled bool
)

type watchStep struct {
	// offset from the session start (cumulative)
	offset time.Duration
	poke   bool
	sample bool
	label  string
}

var visualWatchPlan = []watchStep{
	{45 * time.Second, true, false, "+45s poke"},
	{60 * time.Second, false, true, "+1m sample"},
	{120 * time.Second, true, false, "+2m poke"},
	{180 * time.Second, false, true, "+3m sample"},
	{480 * time.Second, false, true, "+8m sample"},
	{900 * time.Second, false, true, "+15m sample"},
}

// armVisualWatchdog starts one watchdog session. Coalesced: a transition
// while a session runs does not stack a second one — the running session
// re-evaluates the CURRENT desired theme at every step, so it stays valid.
func (a *App) armVisualWatchdog(cause string) {
	visualWatchMu.Lock()
	if visualWatchScheduled {
		visualWatchMu.Unlock()
		diagLog("watchdog: already armed (coalesced, cause=%s)", cause)
		return
	}
	visualWatchScheduled = true
	visualWatchMu.Unlock()
	diagLog("watchdog: armed (%s): pokes at +45s/+2m, samples at +1m/+3m/+8m/+15m", cause)

	go func() {
		// Single exit point for the flag (same lesson as the wake-apply
		// flag): no matter how this session ends, later transitions must be
		// able to arm a fresh one.
		defer func() {
			visualWatchMu.Lock()
			visualWatchScheduled = false
			visualWatchMu.Unlock()
		}()
		prev := time.Duration(0)
		for _, s := range visualWatchPlan {
			sleepForRuntime(s.offset - prev)
			prev = s.offset
			if s.poke {
				diagLog("watchdog[%s]: reinforce poke", s.label)
				broadcastThemeChange()
				pokeTaskbar()
			}
			if s.sample {
				if !a.watchdogSample(s.label) {
					diagLog("watchdog: session aborted at %s", s.label)
					return
				}
			}
		}
		diagLog("watchdog: session complete (all sample points passed)")
	}()
}

// confidentVisualMismatch reports whether the taskbar is CONFIDENTLY showing
// the wrong theme. Anything else (correct, ambiguous mid-tone reading, or an
// unavailable sample) returns false — the watchdog only ever spends a flip
// on a certain mismatch, so it can never cause a false flash.
func confidentVisualMismatch(targetDark bool) (mismatch bool, detail string) {
	med, dump, ok := sampleTaskbarLuma()
	if !ok {
		return false, "sample unavailable (" + dump + ")"
	}
	looksDark := med < 128
	confident := (targetDark && med <= pixelConfidentDark) ||
		(!targetDark && med >= pixelConfidentLight)
	detail = fmt.Sprintf("median=%.0f lumas=[%s] looks=%v confident=%v", med, dump, looksDark, confident)
	return confident && looksDark != targetDark, detail
}

// watchdogSample runs one sample point. Returns false to abort the session
// (the user took control or there is nothing to watch anymore).
func (a *App) watchdogSample(at string) (continueSession bool) {
	cfg := a.GetConfig()
	if flipActive() {
		diagLog("watchdog[%s]: sample skipped (flip in flight)", at)
		return true
	}
	if cfg.ManualTheme != ThemeAuto {
		diagLog("watchdog[%s]: abort (user override %q)", at, cfg.ManualTheme)
		return false
	}
	if !cfg.Enabled {
		diagLog("watchdog[%s]: abort (auto switching disabled)", at)
		return false
	}
	t, err := a.desiredTheme(time.Now())
	if err != nil || t == "" {
		diagLog("watchdog[%s]: abort (desiredTheme unavailable: %v)", at, err)
		return false
	}
	targetDark := t == "dark"
	if currentThemeIsDark() != targetDark {
		// Registry itself is wrong — the 30s tick owns that case; the next
		// sample re-checks the visual state after it lands.
		diagLog("watchdog[%s]: registry mismatch, deferred to tick [%s]", at, themeSnapshot())
		return true
	}
	mismatch, detail := confidentVisualMismatch(targetDark)
	if mismatch {
		diagLog("watchdog[%s]: CONFIDENT VISUAL MISMATCH -> heal: %s", at, detail)
		flipHealTheme(targetDark)
	} else {
		diagLog("watchdog[%s]: not confidently wrong -> reinforce: %s", at, detail)
		broadcastThemeChange()
		pokeTaskbar()
	}
	return true
}
