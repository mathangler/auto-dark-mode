package main

import (
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`

// Win32 constants used for the theme broadcasts.
const (
	HWND_BROADCAST   = 0xffff
	WM_SETTINGCHANGE = 0x001A
	WM_THEMECHANGED  = 0x031A
	SMTO_ABORTIFHUNG = 0x0002
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	uxtheme           = windows.NewLazySystemDLL("uxtheme.dll")
	gdi32dll          = windows.NewLazySystemDLL("gdi32.dll")
	procSendMessageW  = user32.NewProc("SendMessageTimeoutW")
	procPostMessageW  = user32.NewProc("PostMessageW")
	procEnumWindows   = user32.NewProc("EnumWindows")
	procGetClassNameW = user32.NewProc("GetClassNameW")
	// Unbiased interrupt time: unlike GetTickCount64, whose count is biased
	// and keeps advancing through sleep/hibernate, this counter only moves
	// while the machine is actually running. It is what makes the scheduler's
	// resume detection work (see unbiasedInterruptMs).
	procQueryUnbiasedInterruptTime = kernel32.NewProc("QueryUnbiasedInterruptTime")
	procGetTickCount64             = kernel32.NewProc("GetTickCount64")
	// Undocumented uxtheme.dll export: makes the themed-control layer re-read
	// the Personalize registry values immediately instead of relying on its
	// internal cache. Harmless if the export ever disappears (Call just fails).
	procRefreshImmersiveColorPolicyState = uxtheme.NewProc("RefreshImmersiveColorPolicyState")
)

const (
	shellTrayWnd       = "Shell_TrayWnd"
	shellSecondaryTray = "Shell_SecondaryTrayWnd"
)

// unbiasedInterruptMs returns milliseconds of actual machine runtime since
// boot, excluding time spent in sleep/hibernate. GetTickCount64 was used here
// before on the (wrong) assumption that it freezes during sleep — MSDN is
// explicit that its count is biased and includes sleep, so wall-vs-tick
// comparison never detected a resume and the immediate post-wake apply was
// broadcast before the shell was ready (the stuck-taskbar root cause).
// Falls back to GetTickCount64 only on pre-Win7 systems, where the detection
// then simply doesn't fire (the power-event path still covers those).
func unbiasedInterruptMs() uint64 {
	var v uint64
	r, _, _ := procQueryUnbiasedInterruptTime.Call(uintptr(unsafe.Pointer(&v)))
	if r == 0 {
		r2, _, _ := procGetTickCount64.Call()
		return uint64(r2)
	}
	return v / 10000 // 100ns units -> milliseconds
}

// sleepForRuntime waits for d of actual machine runtime. A plain time.Sleep
// can return early from the caller's point of view after a sleep/hibernate:
// the Go runtime's monotonic clock keeps advancing while the process is
// frozen, so a timer that "expired" during sleep fires the instant the
// machine wakes (the biased-interrupt behavior MSDN documents for such
// timers). Waiting out the unelapsed remainder against the unbiased counter
// keeps delays real across a sleep.
func sleepForRuntime(d time.Duration) {
	for d > 0 {
		start := unbiasedInterruptMs()
		time.Sleep(d)
		elapsed := time.Duration(unbiasedInterruptMs()-start) * time.Millisecond
		if elapsed >= d {
			return
		}
		d -= elapsed
		if d > 50*time.Millisecond {
			diagLog("wait: only %s of the requested window ran -> re-waiting remaining %s (machine slept mid-wait or timer fired early)",
				elapsed.Round(time.Millisecond), d.Round(time.Millisecond))
		}
	}
}

// notifyShell sends a single WM_SETTINGCHANGE "ImmersiveColorSet" broadcast.
// Writing the Personalize registry keys is not enough on its own: Explorer and
// the taskbar only repaint dark/light after they receive this notification.
func notifyShell() {
	diagLog("broadcast: WM_SETTINGCHANGE ImmersiveColorSet -> HWND_BROADCAST")
	payload, _ := windows.UTF16FromString("ImmersiveColorSet")
	var result uintptr
	procSendMessageW.Call(
		HWND_BROADCAST,
		WM_SETTINGCHANGE,
		0,
		uintptr(unsafe.Pointer(&payload[0])),
		SMTO_ABORTIFHUNG,
		3000,
		uintptr(unsafe.Pointer(&result)),
	)
}

// Heal worker state. The long tail of extra broadcasts is only used when the
// shell may still be coming up (system login / resume from sleep).
// SendMessageTimeout is synchronous and waits for every top-level window, so
// the tail runs on a dedicated goroutine and never stacks.
var (
	healMu      sync.Mutex
	healRunning bool
	healPending bool
)

// broadcastThemeChange tells the shell to re-read the theme with a single
// broadcast, sent on a goroutine. SendMessageTimeout(HWND_BROADCAST) blocks
// the sender until every top-level window processed the message — which can
// take seconds — so it must never run on the UI/tray thread. The shell itself
// receives the message near the start of the broadcast, so switching stays
// fast. (The long re-broadcast tail for boot/resume is healShellAfterResume.)
func broadcastThemeChange() {
	go notifyShell()
}

// healShellAfterResume schedules extra broadcasts over the next ~77s for when
// the shell may still be coming up (system login / resume from sleep) and can
// miss the immediate notification. Coalesced so repeated triggers never stack.
func healShellAfterResume() {
	healMu.Lock()
	if healRunning {
		healPending = true
		healMu.Unlock()
		diagLog("tail: re-broadcast tail already running -> queued one follow-up")
		return
	}
	healRunning = true
	healMu.Unlock()
	diagLog("tail: re-broadcast tail started (gaps 2/5/10/20/40s)")

	go func() {
		for _, gap := range []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second} {
			// Sleep-aware: a plain sleep here would have every remaining gap
			// "expire" during a subsequent sleep and fire as an immediate
			// broadcast burst at the next wake — exactly when the shell is
			// least able to hear it.
			sleepForRuntime(gap)
			notifyShell()
		}
		healMu.Lock()
		again := healPending
		healPending = false
		healRunning = false
		healMu.Unlock()
		if again {
			healShellAfterResume()
		}
	}()
}

// reapplyCurrentTheme re-writes the current theme and re-broadcasts it. Used
// by manual overrides so a shell that missed an earlier notification gets
// healed even when the registry already matches.
func reapplyCurrentTheme() error {
	return applyThemeHealed(currentThemeIsDark())
}

// refreshImmersiveColorPolicyState asks the themed-control layer to re-read
// the theme registry values. Best-effort: if the export is missing, nothing
// happens.
func refreshImmersiveColorPolicyState() {
	if procRefreshImmersiveColorPolicyState.Find() != nil {
		return
	}
	procRefreshImmersiveColorPolicyState.Call()
}

// writeSystemThemeValues writes only the system (taskbar / Start menu) side
// of the Personalize values, leaving the app-side value untouched. The flip
// heal's intermediate step uses this: the system side is the one whose cache
// desyncs after a resume, and flipping only it forces that side to re-sync
// without every app on screen flashing to the opposite theme first.
func writeSystemThemeValues(dark bool) error {
	value := uint32(1)
	if dark {
		value = 0
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, personalizeKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	for _, name := range []string{"SystemUsesLightTheme", "SystemUseLightTheme"} {
		if err := k.SetDWordValue(name, value); err != nil {
			diagLog("registry: SYSTEM-ONLY write FAILED (dark=%v): %v", dark, err)
			return err
		}
	}
	diagLog("registry: SYSTEM-ONLY dark=%v -> %s", dark, themeSnapshot())
	return nil
}

// writeThemeValues writes the Personalize registry values for the theme
// without broadcasting. AppsUseLightTheme=0 / SystemUsesLightTheme=0 means
// dark; 1 means light.
func writeThemeValues(dark bool) error {
	value := uint32(1)
	if dark {
		value = 0
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, personalizeKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// Keep every variant of the theme keys in sync so no build of Windows
	// reads a leftover value from the wrong key.
	for _, name := range []string{"AppsUseLightTheme", "SystemUsesLightTheme", "SystemUseLightTheme"} {
		if err := k.SetDWordValue(name, value); err != nil {
			diagLog("registry: ALL-keys write FAILED (dark=%v): %v", dark, err)
			return err
		}
	}
	diagLog("registry: ALL keys dark=%v -> %s", dark, themeSnapshot())
	return nil
}

// flipHealPause is how long the opposite theme stays written before the
// target theme is written back. The shell lags behind registry transitions
// (it watches the registry and repaints through a queued pipeline), and when
// the two opposite transitions are only fractions of a second apart the
// second one can be lost — leaving the taskbar on the opposite theme from
// the registry, which is exactly the stuck state being healed. The manual
// "force dark, then force light" sequence that provably heals has the two
// changes seconds apart; the pause copies that rhythm.
const flipHealPause = 3 * time.Second

// Flip heal state. flipRunning means a round trip is in flight (its
// intermediate registry value is intentionally the opposite theme); extra
// requests arriving meanwhile are coalesced into one queued follow-up.
var (
	flipMu         sync.Mutex
	flipRunning    bool
	flipQueued     bool
	flipQueuedDark bool
)

// flipActive reports whether a flip heal is in flight. While it is, the
// scheduler must not write the theme itself: it would race the flip's
// intermediate (opposite) value and can leave the shell between themes.
func flipActive() bool {
	flipMu.Lock()
	defer flipMu.Unlock()
	return flipRunning
}

// flipHealTheme heals a shell whose theme cache is stuck on the registry
// value that is already stored: the taskbar (Shell_TrayWnd) and the Explorer
// file-list view only repaint on WM_SETTINGCHANGE(ImmersiveColorSet) when the
// values actually changed relative to their internal cache, so repeated
// broadcasts of an unchanged theme are ignored (observed after waking from
// sleep during the opposite theme period). The fix is a real flip: write the
// opposite theme and broadcast, wait flipHealPause, then write the target
// theme back and broadcast — forcing the shell cache to re-sync — and poke
// the taskbar window directly as an extra delivery path. Runs on a goroutine
// because each synchronous broadcast can block for seconds on a hung window.
// Concurrent requests are coalesced, and a queued request is only acted on if
// its target differs from what the running flip just wrote — a healed shell
// must never be flipped a second time, because stacking flips is what turns
// a successful heal back into a stuck taskbar.
func flipHealTheme(dark bool) {
	flipMu.Lock()
	if flipRunning {
		flipQueued = true
		flipQueuedDark = dark
		flipMu.Unlock()
		diagLog("flip: request dark=%v coalesced (a flip is already running, queued)", dark)
		return
	}
	flipRunning = true
	flipMu.Unlock()
	diagLog("flip: START dark=%v (system-side round trip, dwell %s)", dark, flipHealPause)

	go func() {
		runFlip(dark)

		flipMu.Lock()
		rerun := flipQueued
		rerunDark := flipQueuedDark
		flipQueued = false
		flipRunning = false
		flipMu.Unlock()

		if rerun && rerunDark != dark {
			// The queued request targets the other theme — a real change is
			// needed, not another flip. (Same target: the flip just converged
			// the shell onto it, so there is nothing left to do.)
			applyTheme(rerunDark)
		}
	}()
}

// runFlip performs one system-side dark↔light round trip ending at the given
// theme. The intermediate step writes only the system (taskbar/Start) values:
// that is the side whose cache desyncs after a resume, so flipping it forces
// the re-sync while every app on screen stays on the target theme — no
// whole-desktop flash. The pause between the two writes uses the
// sleep-aware wait so a machine that falls asleep mid-heal doesn't wake up
// hours later with the registry still parked on the opposite value.
func runFlip(dark bool) {
	if err := writeSystemThemeValues(!dark); err != nil {
		diagLog("flip: step1 (opposite) write FAILED, skipping dwell: %v", err)
	} else {
		diagLog("flip: step1 delivered, dwelling %s with system=%v", flipHealPause, !dark)
		refreshImmersiveColorPolicyState()
		notifyShell()
		pokeTaskbar()
		sleepForRuntime(flipHealPause)
	}
	if err := writeThemeValues(dark); err != nil {
		diagLog("flip: step2 (target) write FAILED: %v", err)
	} else {
		diagLog("flip: target written, final delivery")
		refreshImmersiveColorPolicyState()
		notifyShell()
	}
	// One more targeted delivery in case the broadcast missed the taskbar.
	pokeTaskbar()
	diagLog("flip: DONE dark=%v [%s]", dark, themeSnapshot())
	// Post-restore delivery ladder (B-segment): the RESTORE transition is the
	// one that must not be missed — if the shell drops it, the registry says
	// target while the taskbar sits on the opposite theme and nothing later
	// can correct that (unchanged broadcasts are ignored). Re-deliver the
	// targeted notifications (synchronous + queued PostMessage) a couple of
	// times over the following seconds. Sleep-aware, like every other wait.
	go func() {
		for _, gap := range []time.Duration{2 * time.Second, 5 * time.Second} {
			sleepForRuntime(gap)
			diagLog("flip: ladder re-delivery +%s [%s]", gap, themeSnapshot())
			pokeTaskbar()
		}
	}()
	// The shell may still be busy; keep the slow re-broadcast tail running.
	healShellAfterResume()
	go syncTrayIcon(dark)
}

// postedColorSetParam returns a pointer to the UTF-16 "ImmersiveColorSet"
// string that stays valid for the life of the process (see the once-block
// below for why that matters for a posted message). Returns 0 until
// initialized, in which case callers skip the queued delivery.
var (
	postParamOnce sync.Once
	postParam     uintptr
	postParamBuf  []uint16
)

func postedColorSetParam() uintptr {
	postParamOnce.Do(func() {
		// A posted message's lParam is read by the receiver whenever it gets
		// around to processing it, so it must stay valid forever. The buffer
		// is package-level and never reallocated, and the Go GC does not move
		// heap objects, so &buf[0] stays put for the life of the process.
		buf, _ := windows.UTF16FromString("ImmersiveColorSet")
		postParamBuf = buf
		postParam = uintptr(unsafe.Pointer(&postParamBuf[0]))
	})
	return postParam
}

// pokeTaskbar delivers the theme notifications directly to the taskbar
// windows (primary plus any secondary taskbars on other monitors). Two
// delivery paths on purpose:
//   - SendMessageTimeout reaches a responsive taskbar immediately;
//   - PostMessage queues the message even on a hung/not-yet-resumed one —
//     SendMessageTimeout with SMTO_ABORTIFHUNG silently drops the message for
//     a hung window, and a dropped transition broadcast is exactly how the
//     taskbar gets stuck on the old theme after a resume.
//
// Best effort: windows that can't be found are skipped.
func pokeTaskbar() {
	delivered := 0
	for _, cls := range []string{shellTrayWnd, shellSecondaryTray} {
		for _, hwnd := range findWindowsOfClass(cls) {
			delivered++
			var result uintptr
			payload, _ := windows.UTF16FromString("ImmersiveColorSet")
			procSendMessageW.Call(hwnd, WM_SETTINGCHANGE, 0,
				uintptr(unsafe.Pointer(&payload[0])), SMTO_ABORTIFHUNG, 3000,
				uintptr(unsafe.Pointer(&result)))
			procSendMessageW.Call(hwnd, WM_THEMECHANGED, 0, 0, SMTO_ABORTIFHUNG, 3000,
				uintptr(unsafe.Pointer(&result)))
			if p := postedColorSetParam(); p != 0 {
				procPostMessageW.Call(hwnd, WM_SETTINGCHANGE, 0, p)
			}
		}
	}
	diagLog("poke: delivered to %d taskbar window(s)", delivered)
}

// findWindowsOfClass returns the window handles of all top-level windows of
// the given class (EnumWindows, since FindWindow returns only the first).
// Guarded by a mutex because the enum callback can only be registered once
// and its state is shared between concurrent callers.
var enumMu sync.Mutex

var enumCollect struct {
	cls   string
	found []uintptr
}

var enumWindowsCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	buf := make([]uint16, len(enumCollect.cls)+1)
	n, _, _ := procGetClassNameW.Call(hwnd,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n > 0 && windows.UTF16ToString(buf[:n]) == enumCollect.cls {
		enumCollect.found = append(enumCollect.found, hwnd)
	}
	return 1 // keep enumerating
})

func findWindowsOfClass(cls string) []uintptr {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumCollect.cls = cls
	enumCollect.found = enumCollect.found[:0]
	procEnumWindows.Call(enumWindowsCallback, 0)
	out := make([]uintptr, len(enumCollect.found))
	copy(out, enumCollect.found)
	return out
}

// applyThemeHealed applies the theme, falling back to the flip heal when the
// registry already matches the target. A matching registry means applyTheme
// would only write unchanged values, which the shell provably ignores —
// exactly the stuck-taskbar-after-resume case this heals.
func applyThemeHealed(dark bool) error {
	if currentThemeIsDark() != dark {
		return applyTheme(dark)
	}
	diagLog("heal-apply: registry already matches dark=%v -> flip heal [%s]", dark, themeSnapshot())
	flipHealTheme(dark)
	return nil
}

// refreshShellTheme pokes the shell to re-read the current theme without
// changing anything. Used after a power-resume event so the taskbar etc. don't
// stay on a stale theme.
func refreshShellTheme() {
	broadcastThemeChange()
}

// applyTheme sets Windows' global dark/light mode via the Personalize keys.
// AppsUseLightTheme=0 / SystemUsesLightTheme=0 means dark; 1 means light.
// The shell ignores a silent registry change, so this follows the canonical
// switch order (write → RefreshImmersiveColorPolicyState → broadcast) that
// well-known theme switchers use, then adds a targeted delivery to the
// taskbar windows: SendMessageTimeout silently DROPS the message for a hung
// window, and a hung taskbar right after a resume is exactly how it ends up
// stuck on the old theme — the queued PostMessage reaches it once it pumps
// again. A theme change is always followed by the slow re-broadcast tail,
// NOT just around boot/resume: if the shell is busy or still waking up it
// can miss the very first broadcast (the taskbar and Explorer only repaint
// after receiving it), and the retries at 2/5/10/20/40s catch it once it is
// idle again.
func applyTheme(dark bool) error {
	diagLog("applyTheme: begin dark=%v [%s]", dark, themeSnapshot())
	if err := writeThemeValues(dark); err != nil {
		return err
	}
	refreshImmersiveColorPolicyState()
	broadcastThemeChange()
	// The targeted taskbar delivery can block seconds on a hung window, and
	// this function also runs on the tray thread (via the tray's force-theme
	// menu items) — so the poke runs on a goroutine, like the broadcast.
	go pokeTaskbar()
	healShellAfterResume()
	// Push the tray icon to the applied theme immediately (the tray poller is
	// only a fallback).
	go syncTrayIcon(dark)
	return nil
}

// currentThemeIsDark reads the current OS theme preference.
func currentThemeIsDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return v == 0
}
