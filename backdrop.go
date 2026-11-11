package main

import (
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// DWM system backdrop probing (v2.0.0 Liquid Glass shell).
//
// Wails already asks DWM for an Acrylic backdrop when WindowIsTranslucent is
// set (options.Windows.BackdropType), but it cannot tell us whether the
// material actually took effect: Windows 10, Windows 11 builds before the
// backdrop attribute existed, and "transparency effects = off" all silently
// yield nothing. The UI must then paint an opaque time-of-day gradient
// instead of showing a black window, so this file re-applies the attribute
// and reports the real result to the frontend via State.Backdrop.

const (
	dwmwaUseImmersiveDarkMode   = 20
	dwmwaWindowCornerPreference = 33
	dwmwaSystemBackdropType     = 38
)

var (
	backdropUser32 = syscall.NewLazyDLL("user32.dll")
	backdropDwmapi = syscall.NewLazyDLL("dwmapi.dll")

	procFindWindowW           = backdropUser32.NewProc("FindWindowW")
	procDwmSetWindowAttribute = backdropDwmapi.NewProc("DwmSetWindowAttribute")
	procSetWindowPos          = backdropUser32.NewProc("SetWindowPos")
)

const (
	hwndNotTopmost = ^uintptr(0) - 1 // HWND_NOTOPMOST (-2)
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoActivate  = 0x0010
)

var (
	backdropMu   sync.RWMutex
	backdropKind = "none"
	backdropHwnd uintptr
)

// currentBackdrop reports "acrylic" or "none" for the UI.
func currentBackdrop() string {
	backdropMu.RLock()
	defer backdropMu.RUnlock()
	return backdropKind
}

func setBackdrop(kind string) {
	backdropMu.Lock()
	backdropKind = kind
	backdropMu.Unlock()
}

// probeWindowBackdrop waits for the Wails window to exist, applies the
// Acrylic backdrop, records whether it worked and pushes the result to the UI.
func probeWindowBackdrop(a *App) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if h := findAppWindow(); h != 0 {
			backdropMu.Lock()
			backdropHwnd = h
			backdropMu.Unlock()

			// Never let the settings window sit above other applications:
			// a stray WS_EX_TOPMOST (from a debugging tool, or an older build)
			// makes clicks outside appear to do nothing — the user then has to
			// minimize before another window becomes usable.
			procSetWindowPos.Call(h, hwndNotTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)

			// Rounded corners (Win11); harmless elsewhere.
			corner := int32(2) // DWMWCP_ROUND
			procDwmSetWindowAttribute.Call(h, dwmwaWindowCornerPreference,
				uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))

			kind := "none"
			if transparencyEnabled() {
				bd := int32(3) // DWMSBT_TRANSIENTWINDOW (Acrylic)
				hr, _, _ := procDwmSetWindowAttribute.Call(h, dwmwaSystemBackdropType,
					uintptr(unsafe.Pointer(&bd)), unsafe.Sizeof(bd))
				if hr == 0 {
					kind = "acrylic"
				}
			}
			setBackdrop(kind)
			applyBackdropTheme(currentThemeIsDark())
			diagLog("ui: backdrop=%s transparency=%v", kind, transparencyEnabled())
			a.emitState()
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	setBackdrop("none")
	diagLog("ui: backdrop probe timed out (window not found) -> gradient fallback")
	a.emitState()
}

// applyBackdropTheme keeps the DWM frame in sync with the light/dark switch.
func applyBackdropTheme(dark bool) {
	backdropMu.RLock()
	h := backdropHwnd
	backdropMu.RUnlock()
	if h == 0 {
		return
	}
	v := int32(0)
	if dark {
		v = 1
	}
	procDwmSetWindowAttribute.Call(h, dwmwaUseImmersiveDarkMode,
		uintptr(unsafe.Pointer(&v)), unsafe.Sizeof(v))
}

func findAppWindow() uintptr {
	cls, err := syscall.UTF16PtrFromString("wailsWindow")
	if err != nil {
		return 0
	}
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0)
	return h
}

// transparencyEnabled mirrors Settings > Personalization > Colors >
// Transparency effects. When it is off, DWM refuses to draw any material.
func transparencyEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("EnableTransparency")
	if err != nil {
		return true
	}
	return v != 0
}
