package main

import (
	_ "embed"
	"os"
	"path/filepath"
	gruntime "runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

//go:embed build/tray/sun.ico
var sunIconBytes []byte

//go:embed build/tray/moon.ico
var moonIconBytes []byte

// --- Win32 API handles ---------------------------------------------------

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	kernel  = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClass       = user32.NewProc("RegisterClassExW")
	procCreateWindow        = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procLoadImage           = user32.NewProc("LoadImageW")
	procGetModuleHandle     = kernel.NewProc("GetModuleHandleW")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")

	// Despite the "Power" prefix these are USER functions exported by
	// user32.dll (they deliver WM_POWERBROADCAST to a window handle). The
	// powrprof.dll counterpart PowerSettingRegisterNotification takes an
	// event handle instead — not what we want here.
	procRegisterPowerSettingNotification   = user32.NewProc("RegisterPowerSettingNotification")
	procUnregisterPowerSettingNotification = user32.NewProc("UnregisterPowerSettingNotification")
)

// --- Win32 types & constants ---------------------------------------------

const (
	IMAGE_ICON      = 0x00000001
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040

	WM_USER           = 0x0400
	WM_RBUTTONUP      = 0x0205
	WM_LBUTTONUP      = 0x0202
	WM_POWERBROADCAST = 0x0218

	NIM_ADD     = 0x00000000
	NIM_MODIFY  = 0x00000001
	NIM_DELETE  = 0x00000002
	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	MF_SEPARATOR = 0x00000800
	MF_STRING    = 0x00000000
	MF_CHECKED   = 0x00000008
	MF_UNCHECKED = 0x00000000

	TPM_LEFTALIGN   = 0x00000000
	TPM_BOTTOMALIGN = 0x00000020
	TPM_RETURNCMD   = 0x00000100
	TPM_NONOTIFY    = 0x00000080
)

// Power events. WM_POWERBROADCAST reports resumption through several
// PBT_APMRESUME* codes depending on hardware/power-plan, and Modern Standby
// wakes often arrive only as PBT_POWERSETTINGCHANGE (away-mode transition).
const (
	PBT_APMRESUMECRITICAL  = 0x0006
	PBT_APMRESUMESUSPEND   = 0x0007
	PBT_APMRESUMESTANDBY   = 0x0008
	PBT_APMRESUMEHYBRID    = 0x0009
	PBT_APMRESUMEAUTOMATIC = 0x0012
	PBT_POWERSETTINGCHANGE = 0x8013

	DEVICE_NOTIFY_WINDOW_HANDLE = 0x00000000
)

const (
	actionToggle = 1
	actionOpen   = 2
	actionDark   = 3
	actionLight  = 4
	actionAuto   = 5
	actionQuit   = 6
)

type point struct {
	X, Y int32
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type notifyIconData struct {
	cbSize            uint32
	hWnd              windows.Handle
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	hIcon             windows.Handle
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
	guidItem          windows.GUID
	hBalloonIcon      windows.Handle
}

// powerBroadcastSetting mirrors POWERBROADCAST_SETTING (lParam of the
// WM_POWERBROADCAST / PBT_POWERSETTINGCHANGE message). The setting payload
// follows this header at offset 20 on both 32- and 64-bit.
type powerBroadcastSetting struct {
	powerSetting windows.GUID
	dataLength   uint32
}

// Tray state shared between the run loop, the watchdog and the app.
var (
	trayHwnd   windows.Handle
	trayUID    uint32 = 1
	app        *App
	sunIcon    windows.Handle
	moonIcon   windows.Handle
	trayIconMu sync.Mutex // guards NIM_MODIFY calls
)

// Modern Standby (S0 low-power idle) wakes without any PBT_APMRESUME* event;
// the only signal can be a PBT_POWERSETTINGCHANGE for GUID_SYSTEM_AWAYMODE.
// The data byte flips between 0 and 1 on enter/leave, and only the leave
// (wake) direction triggers a heal. lastWakeHeal coalesces the flurry of
// events that can arrive around a real wake.
var (
	guidSystemAwayMode = windows.GUID{
		Data1: 0x98A7F580,
		Data2: 0x01F7,
		Data3: 0x48AA,
		Data4: [8]byte{0x9C, 0x0F, 0x44, 0x35, 0x2C, 0x29, 0xE5, 0xC0},
	}
	powrNotifyHandle windows.Handle
	awayModeLast     byte

	wakeMu       sync.Mutex
	lastWakeHeal time.Time
)

// trayWndProc is the hidden window procedure that receives tray callbacks.
func trayWndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case WM_USER + 1: // tray icon callback message
		ev := uint32(lParam & 0xffff)
		if ev == WM_RBUTTONUP || ev == WM_LBUTTONUP {
			showTrayMenu(windows.Handle(hwnd))
		}
		return 0
	case WM_POWERBROADCAST:
		switch uint32(wParam) {
		case PBT_APMRESUMECRITICAL, PBT_APMRESUMESUSPEND, PBT_APMRESUMESTANDBY,
			PBT_APMRESUMEHYBRID, PBT_APMRESUMEAUTOMATIC:
			// The shell can miss the theme notification fired right at wake
			// (it is still resuming), leaving the taskbar on the old theme.
			// Once it has finished resuming, force a re-apply + broadcast to
			// heal it and apply whatever the schedule says now.
			diagLog("power: resume event code=0x%X -> scheduleWakeHeal", wParam)
			scheduleWakeHeal()
		case PBT_POWERSETTINGCHANGE:
			// Modern Standby wakes arrive here instead of as a resume event.
			onPowerSettingChange(lParam)
		}
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, message, wParam, lParam)
	return ret
}

// scheduleWakeHeal coalesces the burst of wake notifications (multiple resume
// codes and away-mode transitions can arrive back-to-back) and runs the heal
// shortly after, once the shell has finished resuming.
func scheduleWakeHeal() {
	wakeMu.Lock()
	if time.Since(lastWakeHeal) < 5*time.Second {
		wakeMu.Unlock()
		diagLog("power: wake burst coalesced (within 5s of previous)")
		return
	}
	lastWakeHeal = time.Now()
	wakeMu.Unlock()
	if app == nil {
		return
	}
	diagLog("power: wake heal path armed (markWakeHealing + scheduleWakeApply)")
	// Keep the scheduler re-poking the shell for a couple of minutes even if
	// the registry already matches, so a slow-to-start taskbar still catches up.
	app.markWakeHealing()
	// Apply the target theme once the shell has settled (the delayed apply is
	// coalesced with the scheduler's own resume detection).
	app.scheduleWakeApply()
}

// onPowerSettingChange handles a PBT_POWERSETTINGCHANGE message. Only the
// away-mode transition matters, and only in the wake direction: leaving away
// mode (data → 0) is the resume signal Modern Standby machines deliver
// instead of PBT_APMRESUME*. Entering standby (data → 1) must NOT trigger
// anything: a delayed apply armed at sleep entry would have its timer expire
// while the machine sleeps and fire the instant it wakes — defeating the
// shell-settle delay that the wake apply exists for.
func onPowerSettingChange(lParam uintptr) {
	if lParam == 0 {
		return
	}
	ps := (*powerBroadcastSetting)(dataAt(lParam))
	if ps.powerSetting != guidSystemAwayMode || ps.dataLength < 1 {
		return
	}
	data := *(*byte)(unsafe.Add(dataAt(lParam), unsafe.Sizeof(powerBroadcastSetting{})))
	was := awayModeLast
	awayModeLast = data
	if data == 0 && was != 0 {
		diagLog("power: away-mode LEAVE (wake) -> scheduleWakeHeal")
		scheduleWakeHeal()
	} else if data != was {
		diagLog("power: away-mode ENTER (sleep) -> nothing armed (by design)")
	}
}

// dataAt turns an API-supplied uintptr (a message lParam carrying a pointer)
// back into an unsafe.Pointer without tripping go vet's unsafeptr check.
func dataAt(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}

// registerPowerNotifications asks the OS to deliver away-mode (Modern
// Standby) transitions to the tray window. Best-effort: a missing or
// unavailable API (very old Windows) or a denied registration is simply
// ignored — never panic.
func registerPowerNotifications(hwnd windows.Handle) {
	if hwnd == 0 {
		return
	}
	if err := procRegisterPowerSettingNotification.Find(); err != nil {
		return
	}
	h, _, _ := procRegisterPowerSettingNotification.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&guidSystemAwayMode)),
		DEVICE_NOTIFY_WINDOW_HANDLE)
	if h != 0 {
		powrNotifyHandle = windows.Handle(h)
	}
}

func unregisterPowerNotifications() {
	if powrNotifyHandle != 0 {
		procUnregisterPowerSettingNotification.Call(uintptr(powrNotifyHandle))
		powrNotifyHandle = 0
	}
}

var trayWndProcCallback = syscall.NewCallback(trayWndProc)

// writeTempIcon writes an embedded ICO to a temp file.
func writeTempIcon(data []byte, name string) (string, error) {
	path := filepath.Join(os.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// loadHICON loads an .ico file as a small icon handle.
func loadHICON(path string) windows.Handle {
	ptr, _ := windows.UTF16PtrFromString(path)
	h, _, _ := procLoadImage.Call(
		0, uintptr(unsafe.Pointer(ptr)), IMAGE_ICON, 16, 16, LR_LOADFROMFILE|LR_DEFAULTSIZE)
	return windows.Handle(h)
}

// loadTrayIcons loads both themed icons, retrying transient temp-file errors
// so a failed write can't leave one icon unloaded (which would freeze the tray
// on the wrong theme).
func loadTrayIcons() {
	for attempt := 0; attempt < 3 && (sunIcon == 0 || moonIcon == 0); attempt++ {
		if sunIcon == 0 {
			if p, err := writeTempIcon(sunIconBytes, "autodark_sun.ico"); err == nil {
				sunIcon = loadHICON(p)
			}
		}
		if moonIcon == 0 {
			if p, err := writeTempIcon(moonIconBytes, "autodark_moon.ico"); err == nil {
				moonIcon = loadHICON(p)
			}
		}
		if sunIcon != 0 && moonIcon != 0 {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// currentTrayIcon picks the icon matching the current OS theme.
func currentTrayIcon() windows.Handle {
	if currentThemeIsDark() {
		return moonIcon
	}
	return sunIcon
}

// syncTrayIcon makes the tray icon match the given theme via NIM_MODIFY.
// Safe to call from any goroutine.
func syncTrayIcon(dark bool) {
	trayIconMu.Lock()
	defer trayIconMu.Unlock()
	if trayHwnd == 0 {
		return
	}
	hIcon := sunIcon
	if dark {
		hIcon = moonIcon
	}
	if hIcon == 0 {
		return
	}
	var nid = notifyIconData{
		cbSize: uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:   trayHwnd,
		uID:    trayUID,
		uFlags: NIF_ICON,
		hIcon:  hIcon,
	}
	procShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&nid)))
}

// trayRun runs the tray on its own goroutine with a dedicated message loop.
func trayRun(appRef *App) {
	// Windows requires the tray window to be created and its messages pumped
	// on the SAME OS thread. Pin this goroutine to a thread so the
	// Shell_NotifyIcon callback (e.g. right-click) reaches our message loop.
	gruntime.LockOSThread()
	defer gruntime.UnlockOSThread()
	app = appRef

	hinst, _, _ := procGetModuleHandle.Call(0)
	clsName, _ := windows.UTF16PtrFromString("AutoDarkModeTray")
	var wc = wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   trayWndProcCallback,
		hInstance:     windows.Handle(hinst),
		lpszClassName: clsName,
	}
	procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindow.Call(
		0, uintptr(unsafe.Pointer(clsName)), uintptr(unsafe.Pointer(clsName)),
		0, 0, 0, 0, 0, 0, 0, hinst, 0)
	if hwnd == 0 {
		return
	}
	trayHwnd = windows.Handle(hwnd)
	registerPowerNotifications(trayHwnd)

	// Load both theme icons (with retry so a transient temp-file failure can't
	// freeze the tray on the wrong icon).
	loadTrayIcons()
	if sunIcon == 0 && moonIcon == 0 {
		return
	}

	var nid = notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             windows.Handle(hwnd),
		uID:              trayUID,
		uFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		uCallbackMessage: WM_USER + 1,
		hIcon:            currentTrayIcon(),
	}
	tip, _ := windows.UTF16FromString("Auto Dark Mode")
	copy(nid.szTip[:], tip)
	procShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))

	// Watchdog: the app pushes syncTrayIcon on every theme apply; this loop is
	// a fallback for external theme changes and periodically re-asserts the
	// icon so the shell can't keep a stale cached frame.
	done := make(chan struct{})
	go func() {
		last := currentThemeIsDark()
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		count := 0
		for {
			select {
			case <-done:
				return
			case <-t.C:
				count++
				d := currentThemeIsDark()
				if d != last || count%120 == 0 {
					last = d
					syncTrayIcon(d)
				}
			}
		}
	}()

	// Message loop on this thread (required for popup menus to work).
	var m msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), uintptr(hwnd), 0, 0)
		if ret == 0 || ret == 0xFFFFFFFF {
			break // WM_QUIT or error
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	close(done)
	unregisterPowerNotifications()

	procShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	procDestroyWindow.Call(hwnd)
}

// showTrayMenu builds and shows the context menu at the cursor position.
func showTrayMenu(hwnd windows.Handle) {
	procSetForegroundWindow.Call(uintptr(hwnd))
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	lang := effectiveLang(app.GetConfig())
	enabled := app.GetConfig().Enabled
	hMenu, _, _ := procCreatePopupMenu.Call()

	appendItem := func(id uintptr, text string, checked bool) {
		flags := uintptr(MF_STRING)
		if checked {
			flags |= MF_CHECKED
		}
		t, _ := windows.UTF16PtrFromString(text)
		procAppendMenu.Call(uintptr(hMenu), flags, id, uintptr(unsafe.Pointer(t)))
	}
	appendSep := func() {
		procAppendMenu.Call(uintptr(hMenu), MF_SEPARATOR, 0, 0)
	}

	appendItem(actionToggle, T(lang, "tray.enabled"), enabled)
	appendItem(actionOpen, T(lang, "tray.openSettings"), false)
	appendSep()
	appendItem(actionDark, T(lang, "tray.forceDark"), false)
	appendItem(actionLight, T(lang, "tray.forceLight"), false)
	appendItem(actionAuto, T(lang, "tray.automatic"), false)
	appendSep()
	appendItem(actionQuit, T(lang, "tray.quit"), false)

	cmd, _, _ := procTrackPopupMenu.Call(
		uintptr(hMenu),
		TPM_LEFTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD|TPM_NONOTIFY,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
	procDestroyMenu.Call(uintptr(hMenu))

	switch uintptr(cmd) {
	case actionToggle:
		app.SetEnabled(!enabled)
	case actionOpen:
		app.ShowWindow()
	case actionDark:
		app.SetManualTheme("dark")
	case actionLight:
		app.SetManualTheme("light")
	case actionAuto:
		app.SetManualTheme("auto")
	case actionQuit:
		forceExit = true
		runtime.Quit(app.ctx)
		procPostQuitMessage.Call(0)
	}
}
