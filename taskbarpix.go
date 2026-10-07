package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Taskbar visual sampling (A-segment of the fix). The whole stuck-taskbar
// machinery previously had no way to know what the taskbar actually LOOKED
// like — the automatic heal therefore fired on perfectly healthy shells
// (flashing every theme-changing wake) and nobody could tell a "registry
// wrong" state from a "shell missed the repaint" state. Sampling a small
// pixel grid from the primary taskbar gives exactly that missing fact.
//
// Capture path: BitBlt the taskbar strip into a top-down 32bpp DIB section
// and read it. GetPixel — the obvious API — is NOT exported by user32 on
// this Windows build (verified with GetProcAddress: it returns NULL), and
// x/sys LazyProc.Call PANICS when a procedure is missing; a panic here would
// take the whole app down at heal time. Therefore every procedure used below
// is probed once with Find() and any missing API degrades to "sample
// unavailable", which callers turn into the old blind flip — never into a
// crash and never into "do nothing".
//
// Notes:
//   - The screen DC is what DWM composites, so the values are what the user
//     actually sees.
//   - Icons, the clock and (on transparent taskbars) wallpaper bleed can
//     sway individual samples; a 9x3 grid + median keeps the classification
//     on the bar's background color.
//   - The gate is deliberately conservative: it only skips the flip when the
//     reading is far from the threshold; anything ambiguous falls back to
//     the old behavior (flip), i.e. plan B semantics.
const srCopy = 0x00CC0020

var (
	procGetWindowRect = user32.NewProc("GetWindowRect")
	procGetDC         = user32.NewProc("GetDC")
	procReleaseDC     = user32.NewProc("ReleaseDC")

	procCreateCompatibleDC = gdi32dll.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32dll.NewProc("CreateDIBSection")
	procBitBlt             = gdi32dll.NewProc("BitBlt")
	procSelectObject       = gdi32dll.NewProc("SelectObject")
	procDeleteObject       = gdi32dll.NewProc("DeleteObject")
	procDeleteDC           = gdi32dll.NewProc("DeleteDC")
)

// bitmapInfoHeader is the Win32 BITMAPINFOHEADER (exactly 40 bytes, no
// padding — verified by the field offsets). A negative height requests a
// top-down DIB, so buffer row 0 is the top of the captured strip.
type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type winRect struct {
	left, top, right, bottom int32
}

// pixProbe verifies every procedure the sampler needs, exactly once. Find()
// reports an error instead of panicking, so a future API reshuffle degrades
// the gate instead of crashing the app.
var (
	pixProbeOnce sync.Once
	pixMissing   []string
)

func pixSupported() bool {
	pixProbeOnce.Do(func() {
		check := func(p *windows.LazyProc) {
			if err := p.Find(); err != nil {
				pixMissing = append(pixMissing, p.Name)
			}
		}
		check(procGetWindowRect)
		check(procGetDC)
		check(procReleaseDC)
		check(procCreateCompatibleDC)
		check(procCreateDIBSection)
		check(procBitBlt)
		check(procSelectObject)
		check(procDeleteObject)
		check(procDeleteDC)
		if len(pixMissing) > 0 {
			diagLog("pixel sampler: unavailable, missing procedures: %s (gate degrades to blind flip)",
				strings.Join(pixMissing, ","))
		}
	})
	return len(pixMissing) == 0
}

// sampleTaskbarLuma captures a 9x3 pixel grid from the primary taskbar and
// returns the median luminance (0..255) plus a dump for the log. ok=false
// means the visual state could not be determined.
func sampleTaskbarLuma() (median float64, detail string, ok bool) {
	if !pixSupported() {
		return 0, "capture APIs unavailable (" + strings.Join(pixMissing, ",") + ")", false
	}
	hwinds := findWindowsOfClass(shellTrayWnd)
	if len(hwinds) == 0 {
		return 0, "no Shell_TrayWnd found", false
	}
	var r winRect
	ret, _, _ := procGetWindowRect.Call(hwinds[0], uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return 0, "GetWindowRect failed", false
	}
	w := r.right - r.left
	h := r.bottom - r.top
	if w < 50 || h < 10 || h > 300 || w > 16384 {
		return 0, fmt.Sprintf("implausible taskbar rect %dx%d", w, h), false
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return 0, "GetDC(0) failed", false
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return 0, "CreateCompatibleDC failed", false
	}
	defer procDeleteDC.Call(hdcMem)

	bmi := bitmapInfoHeader{
		size:     40,
		width:    w,
		height:   -h, // top-down
		planes:   1,
		bitCount: 32,
	}
	var bits uintptr
	hbmp, _, _ := procCreateDIBSection.Call(
		hdcMem, uintptr(unsafe.Pointer(&bmi)), 0, /*DIB_RGB_COLORS*/
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbmp == 0 || bits == 0 {
		return 0, "CreateDIBSection failed", false
	}
	// Defers run LIFO: deselect -> delete bitmap -> delete DC -> release DC.
	old, _, _ := procSelectObject.Call(hdcMem, hbmp)
	if old == 0 || old == 0xFFFFFFFF { // HGDI_ERROR
		procDeleteObject.Call(hbmp)
		return 0, "SelectObject failed", false
	}
	defer procSelectObject.Call(hdcMem, old)
	defer procDeleteObject.Call(hbmp)

	blitted, _, _ := procBitBlt.Call(hdcMem, 0, 0, uintptr(w), uintptr(h),
		hdcScreen, uintptr(r.left), uintptr(r.top), srCopy)
	if blitted == 0 {
		return 0, "BitBlt failed", false
	}

	// Read the grid out of the top-down BGRA buffer. Row stride is w*4 and
	// every row starts 4-byte aligned (BITMAPINFO guarantees it for 32bpp).
	var vals []float64
	var parts []string
	rowStride := uintptr(w) * 4
	for _, fx := range []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9} {
		px := int32(float64(w) * fx)
		for _, fy := range []float64{0.2, 0.5, 0.8} {
			py := int32(float64(h) * fy)
			// dataAt (the repo's vet-clean uintptr->Pointer idiom) avoids the
			// unsafeptr diagnostic; the bits pointer addresses a DIB section
			// that lives as long as the bitmap, which the defers guarantee.
			off := uintptr(py)*rowStride + uintptr(px)*4
			v := *(*uint32)(dataAt(bits + off))
			b := float64(v & 0xFF)
			g := float64((v >> 8) & 0xFF)
			rd := float64((v >> 16) & 0xFF)
			l := 0.299*rd + 0.587*g + 0.114*b
			vals = append(vals, l)
			parts = append(parts, fmt.Sprintf("%d", int(l+0.5)))
		}
	}
	dump := strings.Join(parts, ",")
	if len(vals) < 9 {
		return 0, "too few valid pixels: " + dump, false
	}
	sort.Float64s(vals)
	return vals[len(vals)/2], dump, true
}

// Confidence thresholds for skipping the flip. A light bar reads ~200+ and a
// dark bar ~30-80 in practice; requiring >=170 / <=110 keeps a mid-tone or
// transparency-affected reading on the safe side (it flips, as before).
const (
	pixelConfidentDark  = 110.0
	pixelConfidentLight = 170.0
)

// taskbarNeedsFlip reports whether the automatic stuck-heal flip should run,
// based on what the taskbar actually looks like right now. needs=false means
// the bar already shows the target theme — the heal must then reinforce the
// delivery WITHOUT touching the registry (zero flash). When the sample is
// unavailable or ambiguous it returns needs=true: unknown state degrades to
// the previous blind behavior, never to "do nothing".
func taskbarNeedsFlip(targetDark bool) (needs bool, detail string) {
	med, dump, ok := sampleTaskbarLuma()
	if !ok {
		return true, "pixel sample unavailable (" + dump + ") -> blind flip"
	}
	looksDark := med < 128
	confident := (targetDark && med <= pixelConfidentDark) ||
		(!targetDark && med >= pixelConfidentLight)
	base := fmt.Sprintf("median=%.0f lumas=[%s] looks=%v confident=%v", med, dump, looksDark, confident)
	if looksDark == targetDark && confident {
		return false, base + " -> already correct, SKIP flip (poke only)"
	}
	return true, base + " -> flip"
}
