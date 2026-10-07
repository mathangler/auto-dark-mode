package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application backend.
type App struct {
	ctx          context.Context
	mu           sync.Mutex
	cfg          Config
	resolved     LocationResult
	resolvedOnce bool
	lastError    string
	stopCh       chan struct{}

	// Sleep/resume detection: the unbiased interrupt counter
	// (QueryUnbiasedInterruptTime) only advances while the machine is actually
	// running — unlike the wall clock, which keeps running through sleep — so
	// a wall gap far larger than the runtime gap between two scheduler ticks
	// means the machine slept in between. (GetTickCount64 was used here
	// before, but its count is biased and includes sleep time, so this
	// comparison never fired and the theme was applied the instant the
	// machine woke, before the shell could process the broadcast — the root
	// cause of the stuck taskbar.) wakeDeadline keeps the scheduler re-poking
	// the shell for a short while after a resume so a taskbar that missed the
	// wake-time notification catches up.
	tickWall     time.Time
	tickCount    uint64
	wakeDeadline time.Time
	// healingWas mirrors the previous tick's healing-window state so the
	// moment the window CLOSES (the old "recovery cliff": registry matches,
	// broadcasts stop, an unrepainted shell stayed stuck forever) can fire
	// one final targeted poke with the queued PostMessage path.
	healingWas bool
	// resolvedAt stamps the last auto-location resolve attempt; locRetrying
	// prevents stacking background retries. A location error previously
	// latched for the whole process lifetime, leaving auto mode unable to
	// apply anything (another "stuck forever" path).
	resolvedAt  time.Time
	locRetrying bool
	// locJumpPending holds a rejected-but-plausible new fix awaiting a
	// confirming second probe (travel case); see locationJumpLimit.
	locJumpPending *LocationResult
}

// State is a snapshot pushed to the frontend for display.
type State struct {
	Enabled        bool           `json:"enabled"`
	Mode           Mode           `json:"mode"`
	CurrentTheme   string         `json:"currentTheme"`
	DesiredTheme   string         `json:"desiredTheme"`
	ManualTheme    ManualTheme    `json:"manualTheme"`
	NextTransition string         `json:"nextTransition"`
	Location       LocationResult `json:"location"`
	Sunrise        string         `json:"sunrise,omitempty"`
	Sunset         string         `json:"sunset,omitempty"`
	DarkStart      string         `json:"darkStart,omitempty"`
	LightStart     string         `json:"lightStart,omitempty"`
	AutoStart      bool           `json:"autoStart"`
	CloseToTray    bool           `json:"closeToTray"`
	Lang           string         `json:"lang"`
	LastError      string         `json:"lastError,omitempty"`
}

// NewApp creates the application struct.
func NewApp() *App {
	return &App{}
}

// startup loads persisted config and starts the scheduler loop.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.cfg = loadConfig()
	a.resolvedOnce = false
	a.stopCh = make(chan struct{})
	diagLog("startup: version=%s mode=%s enabled=%v manual=%s darkStart=%s lightStart=%s",
		Version, a.cfg.Mode, a.cfg.Enabled, a.cfg.ManualTheme, a.cfg.DarkStart, a.cfg.LightStart)

	// Keep the auto-start Run entry pointing at the current binary (fixes a
	// stale path after the exe was moved/rebuilt) and tidy up if disabled.
	if a.cfg.AutoStart {
		if err := a.setAutoStart(true); err != nil {
			a.setLastError(fmt.Sprintf("%s: %v", T(effectiveLang(a.cfg), "err.autostartFailed"), err))
		}
	} else {
		a.setAutoStart(false)
	}
	startupLog("startup ok, exe=" + exePath())

	// NEVER run the first tick synchronously here: OnStartup executes on the
	// Wails main loop, which also serves the window assets and the JS
	// bindings — anything slow in this function freezes the whole UI (the
	// ~30s black-window bug). The tick itself is now fast (location resolves
	// in the background), but keep it off the critical path regardless.
	go a.runLoop()
	go a.tick() // apply immediately on startup

	// At login the shell may still be starting, so follow up with the slower
	// re-broadcast tail when the app is managing the theme.
	if a.cfg.Enabled || a.cfg.ManualTheme != ThemeAuto {
		healShellAfterResume()
	}
}

// shutdown stops the scheduler loop.
func (a *App) shutdown(ctx context.Context) {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
}

// runLoop checks the schedule periodically.
func (a *App) runLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.tick()
		}
	}
}

// wakeHealWindow is how long after a theme change or a resume the scheduler
// keeps re-broadcasting the current theme every tick, even when the registry
// already matches. The shell may be busy resuming and miss the first few
// notifications; these slow retries over a few minutes cover that.
const wakeHealWindow = 3 * time.Minute

// locationRetry is how often a FAILED auto-location resolve is retried, in
// the background: resolveLocation can take tens of seconds (GPS probe plus
// two HTTP geolocation services), so the retry must never run on the
// scheduler's path.
const locationRetry = 10 * time.Minute

// locationRefresh is the cadence for refreshing a SUCCESSFUL cache. The
// first draft reused locationRetry (10min) here and produced a 24/7 probe
// every ~10.5 minutes — 63 GPS subprocess spawns in a single day (observed).
// Real locations change rarely; 6 hours bounds it to ~4 probes/day.
const locationRefresh = 6 * time.Hour

// Coordinate-jump protection: a single bad fix (observed: Windows Location
// handing back coordinates ~10° west — roughly 1000 km off — for over two
// hours)
// silently moves the sunrise/sunset boundary by ~50 minutes. A jump beyond
// locationJumpLimit degrees is rejected and the old coordinates are kept;
// the retry runs at the locationRetry cadence, and a second probe agreeing
// with the rejected fix within locationJumpConfirm degrees accepts both
// (the machine really moved — travel).
const (
	locationJumpLimit   = 2.0 // degrees (~200 km)
	locationJumpConfirm = 0.5 // degrees
)

// tick applies the theme that should be active right now.
func (a *App) tick() {
	a.mu.Lock()
	cfg := a.cfg
	now := time.Now()
	lastWall := a.tickWall
	lastCount := a.tickCount
	a.tickWall = now
	a.tickCount = unbiasedInterruptMs()

	// Resume from sleep/hibernate: the wall clock advanced far past the
	// runtime counter, which only moves while the machine is actually
	// running. Detected here in the scheduler, so it works even when the tray
	// window receives no WM_POWERBROADCAST.
	wallGap := now.Sub(lastWall)
	runtimeGap := time.Duration(a.tickCount-lastCount) * time.Millisecond
	resumed := !lastWall.IsZero() && resumeDetected(wallGap, runtimeGap)
	if resumed {
		a.wakeDeadline = now.Add(wakeHealWindow)
	}
	a.mu.Unlock()

	if resumed {
		diagLog("tick: RESUME detected (wallGap=%s runtimeGap=%s) -> arm wake apply, NO registry write now [%s]",
			wallGap.Round(time.Second), runtimeGap.Round(time.Second), themeSnapshot())
		// Do NOT write the theme right now. The shell is still coming up and
		// can drop the broadcast that accompanies a real registry transition
		// — leaving the taskbar on the old theme, which nothing later can fix
		// (broadcasts with unchanged values are ignored). Instead, wait for
		// the shell to settle and apply the target theme once; the wake's
		// only real transition then happens where the shell can hear it.
		a.scheduleWakeApply()
		return
	}

	// A burst of overdue ticker deadlines fires back-to-back the instant the
	// machine resumes (Go timers whose deadline passed during sleep all run
	// immediately). The FIRST tick of the burst takes the branch above; the
	// SECOND one has a tiny wall gap, is correctly read as "not a resume" —
	// and would then take this normal path, applying the theme at wake+0.5s.
	// That is exactly what wakeApplyDelay exists to prevent: the shell is
	// still coming up (proven: poke found ZERO taskbar windows at wake+5s),
	// so the real transition broadcast lands nowhere, and the later wake
	// apply then sees "no transition" and never arms the safety-net flip —
	// the taskbar stays stuck while only unchanged-value broadcasts fly.
	// (app.log 2026-09-23 00:37:03, forensic build.) While the wake apply is
	// still pending, every tick defers to it; once it has run, the registry
	// already matches and ticks degrade to harmless re-broadcasts.
	wakeApplyMu.Lock()
	wakePending := wakeApplyScheduled
	wakeApplyMu.Unlock()
	if wakePending {
		diagLog("tick: deferred, wake apply pending [%s]", themeSnapshot())
		return
	}

	target := string(cfg.ManualTheme)
	if target == "auto" {
		if !cfg.Enabled {
			return
		}
		var err error
		target, err = a.desiredTheme(now)
		if err != nil {
			diagLog("tick: desiredTheme error, nothing applied: %v", err)
			a.setLastError(err.Error())
			a.emitState()
			return
		}
	}
	if target == "" {
		return
	}
	dark := target == "dark"
	if flipActive() {
		diagLog("tick: skip, flip heal in flight [%s]", themeSnapshot())
		// A flip heal is mid-flight: its registry value is intentionally the
		// opposite theme right now, and applying the schedule here would race
		// the flip and can leave the shell between themes. The flip converges
		// on its target; the next tick re-checks.
		return
	}
	a.mu.Lock()
	healing := now.Before(a.wakeDeadline)
	cliff := a.healingWas && !healing
	a.healingWas = healing
	a.mu.Unlock()
	if dark != currentThemeIsDark() {
		diagLog("tick: mismatch -> applyTheme dark=%v [%s]", dark, themeSnapshot())
		if err := applyTheme(dark); err != nil {
			diagLog("tick: applyTheme FAILED: %v", err)
			a.setLastError(fmt.Sprintf("%s: %v", T(a.lang(), "err.applyFailed"), err))
		} else {
			// A real theme switch just happened. applyTheme already started the
			// 2/5/10/20/40s re-broadcast tail; also open the slower per-tick
			// re-broadcast window so the taskbar/Explorer get more chances to
			// repaint even if they were busy (e.g. right after waking up).
			a.markWakeHealing()
			// Start the visual watchdog: this transition can still be undone
			// by a stale shell repaint minutes later — pokes at +45s/+2m,
			// samples at +1m/+3m/+8m/+15m (see watchdog.go).
			a.armVisualWatchdog("tick real transition")
		}
		a.emitState()
	} else if healing {
		// Just resumed or switched and the registry already matches: the shell
		// may still be catching up, so poke it again instead of doing nothing.
		// (A stuck shell ignores unchanged-value broadcasts; healing that case
		// is the user's single-click force apply — the automatic flip was
		// removed because it also fired on healthy shells, flashing every
		// theme-changing wake for nothing.)
		diagLog("tick: healing window, registry matches -> re-broadcast [%s]", themeSnapshot())
		broadcastThemeChange()
		// The plain broadcast has NO queued fallback (SendMessageTimeout with
		// SMTO_ABORTIFHUNG silently drops for a hung taskbar), so pair it
		// with the targeted poke, whose PostMessage survives a hung shell.
		go pokeTaskbar()
		a.emitState()
	} else if cliff {
		// End of the healing window: this is the last automatic chance to
		// reach a shell that never repainted — after this the registry
		// matches, nothing more is scheduled, and an unchanged broadcast
		// would be ignored forever. One final queued-capable poke closes
		// that cliff.
		diagLog("tick: healing window ENDED -> final targeted poke [%s]", themeSnapshot())
		go pokeTaskbar()
	}
}

// wakeApplyDelay is how long after a resume the theme is applied. The shell
// needs a few seconds after a hard resume before it reliably processes theme
// broadcasts; a real registry transition broadcast before then is what leaves
// the taskbar stuck on the old theme.
const wakeApplyDelay = 8 * time.Second

// Coalescing for the delayed wake apply: the burst of wake signals (power
// events, away-mode transitions, scheduler detection) must produce exactly
// one apply. First schedule wins — later signals don't push the apply back.
var (
	wakeApplyMu        sync.Mutex
	wakeApplyScheduled bool
)

// scheduleWakeApply applies the target theme once, wakeApplyDelay after the
// resume, when the shell is ready again. If the theme changed while the
// machine slept, this is the wake's single real registry transition — sent
// to a shell that can actually hear it, so the taskbar follows and the stuck
// state never forms. If the theme didn't change, the apply is a harmless
// unchanged-value write + broadcast. The wait itself is sleep-aware: a
// plain sleep here would have its timer expire during a subsequent sleep and
// fire the instant the machine wakes, defeating the delay.
func (a *App) scheduleWakeApply() {
	wakeApplyMu.Lock()
	if wakeApplyScheduled {
		wakeApplyMu.Unlock()
		diagLog("wake apply: already armed (coalesced)")
		return
	}
	wakeApplyScheduled = true
	wakeApplyMu.Unlock()
	diagLog("wake apply: armed, fires in %s [%s]", wakeApplyDelay, themeSnapshot())

	go func() {
		// Single exit point for the coalescing flag: it must drop after the
		// apply no matter how this goroutine ends, because ticks defer to it
		// while it is set — a stuck flag would freeze all future applies.
		defer func() {
			wakeApplyMu.Lock()
			wakeApplyScheduled = false
			wakeApplyMu.Unlock()
		}()
		sleepForRuntime(wakeApplyDelay)
		diagLog("wake apply: %s elapsed -> running applyWakeTheme", wakeApplyDelay)
		a.applyWakeTheme()
	}()
}

// applyWakeTheme performs the delayed wake-time apply: the theme the override
// or the schedule calls for right now. Skipped while a user-initiated flip
// heal is in flight (its registry value is intentionally the opposite theme).
// When the apply is a REAL transition — the theme changed across the sleep —
// the taskbar may still have missed its broadcast on a busy resume, so a
// one-shot taskbar-side heal is scheduled as a safety net.
func (a *App) applyWakeTheme() {
	cfg := a.GetConfig()
	target := string(cfg.ManualTheme)
	if target == "auto" {
		if !cfg.Enabled {
			diagLog("wake apply: auto disabled -> skip")
			return
		}
		t, err := a.desiredTheme(time.Now())
		if err != nil {
			diagLog("wake apply: desiredTheme error -> skip: %v", err)
			a.setLastError(err.Error())
			a.emitState()
			return
		}
		target = t
	}
	if target == "" || flipActive() {
		diagLog("wake apply: skip (target=%q, flipActive=%v)", target, flipActive())
		return
	}
	dark := target == "dark"
	before := currentThemeIsDark()
	diagLog("wake apply: target=%s before=%v [%s]", target, before, themeSnapshot())
	if err := applyTheme(dark); err != nil {
		diagLog("wake apply: applyTheme FAILED: %v", err)
		a.setLastError(fmt.Sprintf("%s: %v", T(a.lang(), "err.applyFailed"), err))
	} else if before != dark {
		// The wake's theme transition just landed. Normally the shell heard
		// it; if it didn't, only a real value flip can force the taskbar back
		// in sync, and the manual two-click remedy proves that works once the
		// shell is interactive again.
		diagLog("wake apply: applied dark=%v REAL transition -> schedule stuck heal (+%s) [%s]", dark, stuckHealDelay, themeSnapshot())
		a.scheduleStuckHeal(dark)
		a.armVisualWatchdog("wake real transition")
	} else {
		diagLog("wake apply: applied dark=%v no transition [%s]", dark, themeSnapshot())
	}
	a.emitState()
}

// stuckHealDelay is how long after the delayed wake apply the safety-net
// taskbar-side flip runs. It must be well past the resume itself: flips in
// the first seconds after wake are processed by a shell that is not ready
// and were observed to leave the taskbar just as stuck as before. ~45s is
// comfortably inside the window in which the manual force-dark-then-light
// sequence was observed to heal.
const stuckHealDelay = 45 * time.Second

// Coalescing for the stuck-heal: one wake, one heal.
var (
	stuckHealMu        sync.Mutex
	stuckHealScheduled bool
)

// scheduleStuckHeal queues the one-shot taskbar-side flip for a wake whose
// theme actually changed across the sleep — the only situation where the
// taskbar can be left on the old theme with nothing able to correct it.
// The flip touches only the system-side theme values, so applications never
// repaint to the opposite theme while it runs; on a shell that DID hear the
// wake-time transition, the only cost is the taskbar briefly switching and
// switching back.
func (a *App) scheduleStuckHeal(dark bool) {
	stuckHealMu.Lock()
	if stuckHealScheduled {
		stuckHealMu.Unlock()
		diagLog("stuck heal: already armed (coalesced)")
		return
	}
	stuckHealScheduled = true
	stuckHealMu.Unlock()
	diagLog("stuck heal: armed, fires in %s (target dark=%v)", stuckHealDelay, dark)

	go func() {
		sleepForRuntime(stuckHealDelay)
		stuckHealMu.Lock()
		stuckHealScheduled = false
		stuckHealMu.Unlock()
		a.runStuckHeal(dark)
	}()
}

// runStuckHeal performs the delayed flip, but only if the situation still
// calls for it: the registry still holds the theme the wake applied (a user
// override or a later scheduled transition cancels it) and no other flip is
// in flight.
func (a *App) runStuckHeal(dark bool) {
	if flipActive() {
		diagLog("stuck heal: CANCELLED (another flip in flight)")
		return
	}
	if currentThemeIsDark() != dark {
		diagLog("stuck heal: CANCELLED (registry moved to [%s], expected dark=%v)", themeSnapshot(), dark)
		return
	}
	cfg := a.GetConfig()
	target := string(cfg.ManualTheme)
	if target != "auto" {
		if (target == "dark") != dark {
			diagLog("stuck heal: CANCELLED (user forced %q since the wake)", target)
			return // the user forced a different theme since the wake
		}
	} else if !cfg.Enabled {
		diagLog("stuck heal: CANCELLED (auto switching disabled)")
		return // auto with switching disabled: nothing calls for a heal
	} else if t, err := a.desiredTheme(time.Now()); err != nil || t == "" || (t == "dark") != dark {
		diagLog("stuck heal: CANCELLED (schedule moved on since the wake)")
		return // the schedule moved on since the wake
	}
	// A-segment pixel gate: the flip's intermediate dark state is invisible
	// to us, so previously it fired even when the taskbar was already
	// correct (the light->dark->light flash on every theme-changing wake).
	// Sample the bar first; skip the registry round trip entirely when it
	// already shows the target, reinforcing delivery instead.
	if needs, detail := taskbarNeedsFlip(dark); !needs {
		diagLog("stuck heal: pixel gate PASSED (%s) -> reinforce without flip", detail)
		broadcastThemeChange()
		go pokeTaskbar()
		healShellAfterResume()
		go syncTrayIcon(dark)
		return
	} else {
		diagLog("stuck heal: pixel gate (%s)", detail)
	}
	diagLog("stuck heal: gates passed -> flipHealTheme dark=%v [%s]", dark, themeSnapshot())
	flipHealTheme(dark)
}

// markWakeHealing arms the short "re-poke the shell" window after a resume or
// theme switch, so the scheduler keeps broadcasting even when the theme already
// matches. Called from the tray's power-event path and after theme changes.
func (a *App) markWakeHealing() {
	a.mu.Lock()
	a.wakeDeadline = time.Now().Add(wakeHealWindow)
	a.mu.Unlock()
	diagLog("healing window: armed for %s (per-tick re-broadcast)", wakeHealWindow)
}

// resumeDetected reports whether the wall-clock gap and the runtime gap
// (unbiased interrupt time, which freezes during sleep/hibernate) across two
// scheduler ticks indicate the machine slept in between. Any 30+ seconds of
// wall time with no matching runtime is a sleep; a long but proportional gap
// (e.g. the scheduler was merely blocked) is NOT a resume.
func resumeDetected(wallGap, runtimeGap time.Duration) bool {
	return wallGap-runtimeGap > 30*time.Second
}

// currentLocation returns the active resolved location, caching "auto" results.
//
// NEVER resolves the auto path synchronously: the GPS+IP chain can take tens
// of seconds, this function holds a.mu, and it is reached from the startup
// tick on the Wails main loop — a sync resolve there starved the AssetServer
// and every binding, i.e. the "window black / UI unresponsive for ~30s" bug.
// Instead: answer from the in-memory value (seeded from the disk cache) and
// kick a background resolve whenever one is due.
func (a *App) currentLocation() LocationResult {
	a.mu.Lock()
	cfg := a.cfg
	lang := effectiveLang(cfg)
	switch cfg.LocationSource {
	case SourceManual:
		a.mu.Unlock()
		return LocationResult{Lat: cfg.Lat, Lon: cfg.Lon, Source: "manual", Name: "Manual"}
	case SourceCity:
		a.mu.Unlock()
		if c, ok := cityByName(cfg.City); ok {
			return LocationResult{Lat: c.Lat, Lon: c.Lon, Source: "city", Name: c.Name, NameZh: c.NameZh}
		}
		return LocationResult{Error: T(lang, "err.unknownCity")}
	default: // auto
		if !a.resolvedOnce {
			a.resolvedOnce = true
			if c, ok := loadLocationCache(); ok {
				a.resolved = c.LocationResult
				a.resolvedAt = c.At
				diagLog("location: loaded disk cache source=%s at=%s lat=%.5f lon=%.5f",
					c.Source, c.At.Format("2006-01-02 15:04"), c.Lat, c.Lon)
			}
		}
		// Decide if a background resolve is due. Rules:
		//   - nothing in memory            -> cold start, must probe
		//   - last attempt failed          -> retry at locationRetry cadence
		//   - cache older than the cadence -> refresh (a failed refresh KEEPS
		//     the good cached coordinates and only postpones the next try)
		reason := ""
		if a.resolved.Source == "" {
			// Guard with locRetrying like every other branch: without it each
			// currentLocation call during the cold-start probe spawned ANOTHER
			// background resolve (observed: 3× "background resolve done" in
			// 0.4s — tick + emitState + GetState all raced into the gap).
			if !a.locRetrying {
				reason = "cold start"
			}
		} else if a.resolved.Error != "" {
			if !a.locRetrying && (a.resolvedAt.IsZero() || time.Since(a.resolvedAt) >= locationRetry) {
				reason = "previous resolve failed"
			}
		} else if !a.locRetrying && !a.resolvedAt.IsZero() && time.Since(a.resolvedAt) >= locationRefresh {
			reason = "stale cache refresh"
		}
		if reason != "" {
			a.locRetrying = true
			prevGood := a.resolved.Source != "" && a.resolved.Error == ""
			go func() {
				diagLog("location: background resolve start (%s)", reason)
				r := a.resolveLocation()
				a.mu.Lock()
				a.resolvedAt = time.Now()
				a.locRetrying = false
				// Success + a healthy previous fix: apply the jump guard.
				adopt := true
				if r.Error == "" && prevGood && a.resolved.Source != "" && a.resolved.Error == "" {
					if d := coordJumpDegrees(a.resolved, r); d > locationJumpLimit {
						if a.locJumpPending != nil && coordJumpDegrees(*a.locJumpPending, r) <= locationJumpConfirm {
							diagLog("location: coordinate jump %.2f deg CONFIRMED by second fix -> adopting (prev=%.4f,%.4f new=%.4f,%.4f)",
								d, a.resolved.Lat, a.resolved.Lon, r.Lat, r.Lon)
							a.locJumpPending = nil
						} else {
							adopt = false
							p := r
							a.locJumpPending = &p
							// Make the confirming probe come in locationRetry
							// (not the full refresh interval).
							a.resolvedAt = time.Now().Add(locationRetry - locationRefresh)
							diagLog("location: coordinate jump %.2f deg REJECTED (prev=%.4f,%.4f new=%.4f,%.4f) -> keeping old coords, confirm probe in %s",
								d, a.resolved.Lat, a.resolved.Lon, r.Lat, r.Lon, locationRetry)
						}
					} else {
						a.locJumpPending = nil
					}
				}
				if adopt && (r.Error == "" || !prevGood) {
					a.resolved = r
				}
				res := a.resolved
				a.mu.Unlock()
				switch {
				case r.Error != "":
					diagLog("location: background resolve failed (kept good cache=%v): %s [kept lat=%.5f lon=%.5f]",
						prevGood, r.Error, res.Lat, res.Lon)
				case !adopt:
					// the rejection was already logged under the mutex
				default:
					saveLocationCache(res)
					diagLog("location: background resolve done source=%s lat=%.5f lon=%.5f",
						res.Source, res.Lat, res.Lon)
				}
				// Push the new coordinates to the open settings window.
				a.emitState()
			}()
		}
		res := a.resolved
		a.mu.Unlock()
		if res.Source == "" && res.Error == "" {
			// Cold start, probe still running: report "no decision yet"
			// (Error path) rather than silently computing from 0,0 — the
			// registry keeps the last theme until the probe lands, and the
			// next tick (or the emitState above) picks the result up.
			return LocationResult{Error: T(lang, "err.noLocation")}
		}
		return res
	}
}

// lang returns the effective display language for the current config.
func (a *App) lang() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return effectiveLang(a.cfg)
}

// --- Wails bindings ---

// GetConfig returns a copy of the current config.
func (a *App) GetConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// SaveConfig validates and persists the given settings, then re-applies.
func (a *App) SaveConfig(cfg Config) error {
	if cfg.DarkStart == "" {
		cfg.DarkStart = "19:00"
	}
	if cfg.LightStart == "" {
		cfg.LightStart = "07:00"
	}
	if cfg.ManualTheme == "" {
		cfg.ManualTheme = ThemeAuto
	}
	if cfg.LocationSource == "" {
		cfg.LocationSource = SourceAuto
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeSolar
	}
	a.mu.Lock()
	prev := a.cfg
	a.cfg = cfg
	if prev.LocationSource != cfg.LocationSource ||
		prev.Lat != cfg.Lat || prev.Lon != cfg.Lon || prev.City != cfg.City {
		// Location settings changed — drop the cached fix so "auto" re-resolves.
		a.resolvedOnce = false
		a.resolved = LocationResult{}
	}
	a.lastError = ""
	a.mu.Unlock()

	if err := cfg.save(); err != nil {
		return err
	}
	diagLog("config saved: mode=%s enabled=%v manual=%s darkStart=%s lightStart=%s",
		cfg.Mode, cfg.Enabled, cfg.ManualTheme, cfg.DarkStart, cfg.LightStart)
	if err := a.setAutoStart(cfg.AutoStart); err != nil {
		a.setLastError(fmt.Sprintf("%s: %v", T(effectiveLang(cfg), "err.autostartFailed"), err))
	}
	a.tick()
	a.emitState()
	return nil
}

// GetLang returns the effective display language ("en" or "zh").
func (a *App) GetLang() string {
	return effectiveLang(a.GetConfig())
}

// SetLang changes the UI language. "" follows the system, otherwise "en"/"zh".
func (a *App) SetLang(lang string) error {
	if lang != "" && lang != "en" && lang != "zh" {
		return fmt.Errorf(T(effectiveLang(a.GetConfig()), "err.invalidTheme"), lang)
	}
	a.mu.Lock()
	a.cfg.Lang = lang
	cfg := a.cfg
	a.mu.Unlock()
	if err := cfg.save(); err != nil {
		return err
	}
	a.emitState()
	return nil
}

// SetEnabled toggles automatic switching on/off.
func (a *App) SetEnabled(enabled bool) error {
	a.mu.Lock()
	a.cfg.Enabled = enabled
	cfg := a.cfg
	a.mu.Unlock()
	diagLog("user: enabled -> %v", enabled)
	if err := cfg.save(); err != nil {
		return err
	}
	a.tick()
	a.emitState()
	return nil
}

// SetCloseTray persists the close-to-tray preference immediately.
func (a *App) SetCloseTray(enabled bool) error {
	a.mu.Lock()
	a.cfg.CloseToTray = enabled
	cfg := a.cfg
	a.mu.Unlock()
	if err := cfg.save(); err != nil {
		return err
	}
	a.emitState()
	return nil
}

// SetAutoStart persists and applies the launch-at-login preference immediately.
func (a *App) SetAutoStart(enabled bool) error {
	a.mu.Lock()
	a.cfg.AutoStart = enabled
	cfg := a.cfg
	a.mu.Unlock()
	if err := cfg.save(); err != nil {
		return err
	}
	if err := a.setAutoStart(enabled); err != nil {
		a.setLastError(fmt.Sprintf("%s: %v", T(effectiveLang(cfg), "err.autostartFailed"), err))
	}
	a.emitState()
	return nil
}

// SetManualTheme forces dark/light, or "auto" to resume the schedule.
func (a *App) SetManualTheme(theme string) error {
	if theme != "auto" && theme != "dark" && theme != "light" {
		return fmt.Errorf(T(a.lang(), "err.invalidTheme"), theme)
	}
	a.mu.Lock()
	a.cfg.ManualTheme = ManualTheme(theme)
	cfg := a.cfg
	a.mu.Unlock()
	diagLog("user: manualTheme -> %s [%s]", theme, themeSnapshot())
	if err := cfg.save(); err != nil {
		return err
	}
	// Always force a re-apply (even when the mode didn't change) so this also
	// heals a shell that missed an earlier theme notification.
	a.forceApplyTheme()
	return nil
}

// forceApplyTheme re-applies the active theme at user request (tray menu or
// settings UI) so a shell that missed an earlier notification gets healed.
// "auto" recomputes from the schedule. When the registry already matches the
// target, applyThemeHealed runs the flip-style heal — the shell provably
// ignores broadcasts whose values did not change, and the flip is the one
// known way to force its cache back in sync (the manual "dark then light"
// two-click sequence, automated into one click).
func (a *App) forceApplyTheme() {
	cfg := a.GetConfig()
	switch cfg.ManualTheme {
	case "dark":
		applyThemeHealed(true)
	case "light":
		applyThemeHealed(false)
	default: // auto: recalculate from the schedule
		if cfg.Enabled {
			if t, err := a.desiredTheme(time.Now()); err == nil && t != "" {
				applyThemeHealed(t == "dark")
			} else {
				reapplyCurrentTheme()
			}
		} else {
			reapplyCurrentTheme()
		}
	}
	a.emitState()
}

// DetectLocation re-runs location resolution and returns the result, emitting
// progress events so the UI can show each stage (permission → position → IP).
func (a *App) DetectLocation() LocationResult {
	a.mu.Lock()
	a.resolvedOnce = true
	lang := effectiveLang(a.cfg)
	a.mu.Unlock()
	ctx := context.Background()

	// Request permission first, with clear feedback at every step.
	a.emitLocating("permission")
	allowed, gpsErr := gpsRequestAccess(ctx)
	if gpsErr == nil {
		if allowed {
			a.emitLocating("getting")
			if gps, err := gpsGetPosition(ctx); err == nil {
				return a.finishDetect(gps)
			}
			a.emitLocating("ipFallback")
		} else {
			a.emitLocating("ipDenied")
		}
	} else {
		a.emitLocating("ipFallback")
	}
	if ip, ipErr := locateByIP(ctx); ipErr == nil {
		return a.finishDetect(ip)
	}
	return a.finishDetect(LocationResult{Error: T(lang, "err.noLocation")})
}

func (a *App) finishDetect(loc LocationResult) LocationResult {
	a.mu.Lock()
	a.resolved = loc
	a.resolvedAt = time.Now()
	a.locJumpPending = nil // an explicit user-triggered detect wins outright
	a.mu.Unlock()
	saveLocationCache(loc)
	diagLog("location: manual detect source=%s lat=%.5f lon=%.5f err=%q",
		loc.Source, loc.Lat, loc.Lon, loc.Error)
	a.emitState()
	return loc
}

// emitLocating pushes a location-progress stage to the frontend.
func (a *App) emitLocating(stage string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "locating", map[string]string{"stage": stage})
}

// GetCities returns the built-in city picker table.
func (a *App) GetCities() []City {
	return cities
}

// GetState builds the current display snapshot.
func (a *App) GetState() State {
	return a.buildState()
}

func (a *App) buildState() State {
	a.mu.Lock()
	cfg := a.cfg
	le := a.lastError
	a.mu.Unlock()

	now := time.Now()
	st := State{
		Enabled:      cfg.Enabled,
		Mode:         cfg.Mode,
		CurrentTheme: themeLabel(currentThemeIsDark()),
		ManualTheme:  cfg.ManualTheme,
		DarkStart:    cfg.DarkStart,
		LightStart:   cfg.LightStart,
		AutoStart:    cfg.AutoStart,
		CloseToTray:  cfg.CloseToTray,
		Lang:         effectiveLang(cfg),
		Location:     a.currentLocation(),
		LastError:    le,
	}
	if target, err := a.desiredTheme(now); err != nil {
		st.LastError = err.Error()
	} else {
		st.DesiredTheme = target
	}
	if n := a.nextTransition(now); !n.IsZero() {
		st.NextTransition = n.Format("2006-01-02 15:04")
	}
	if cfg.Mode == ModeSolar && st.Location.Error == "" {
		sr, ss := sunTimes(st.Location.Lat, st.Location.Lon, now)
		st.Sunrise = sr.Format("15:04")
		st.Sunset = ss.Format("15:04")
	}
	return st
}

func themeLabel(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// emitState pushes a fresh State snapshot to the frontend.
func (a *App) emitState() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "state", a.buildState())
}

// setLastError is stored under the mutex for simplicity.
func (a *App) setLastError(msg string) {
	a.mu.Lock()
	a.lastError = msg
	a.mu.Unlock()
}

// ShowWindow brings the settings window to the front (used by the tray).
func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
}
