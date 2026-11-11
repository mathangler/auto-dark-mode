package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	woptions "github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// forceExit is set when the user quits from the tray, letting OnBeforeClose
// allow a full shutdown even when "close to tray" is enabled.
var forceExit bool

func main() {
	startupLog("booting, exe=" + exePath())
	// Only allow one instance — a tray utility must not be duplicated.
	if !acquireSingleInstance() {
		startupLog("single-instance mutex already held; exiting")
		return
	}

	// Create an instance of the app structure
	app := NewApp()

	// Run the system tray on its own goroutine. It owns a hidden window and
	// its own message loop, so it coexists with Wails' main loop.
	go trayRun(app)

	// Create application with options.
	//
	// v2.0.0 "Liquid Glass" shell: the window is frameless and translucent so
	// the DWM Acrylic system backdrop can show through, with our own title bar
	// drawn in HTML. Wails keeps WS_THICKFRAME (edge resizing) and, with
	// DisableFramelessWindowDecorations=false, the Windows 11 rounded corners
	// and drop shadow. Dragging goes through WM_NCLBUTTONDOWN/HTCAPTION, so
	// native Aero Snap (drag to an edge / to the top) keeps working.
	//
	// The Acrylic backdrop only exists on Windows 11 with transparency effects
	// on; probeWindowBackdrop() verifies it actually took effect and the
	// frontend falls back to an opaque time-of-day gradient when it did not.
	err := wails.Run(&options.App{
		Title:            "Auto Dark Mode",
		Width:            980,
		Height:           700,
		MinWidth:         820,
		MinHeight:        560,
		Frameless:        true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Windows: &woptions.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			BackdropType:                      woptions.Acrylic,
			DisableFramelessWindowDecorations: false,
		},
		OnStartup: app.startup,
		OnShutdown: app.shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			// A real quit from the tray must be allowed to shut down fully.
			if forceExit {
				return false
			}
			// Close-to-tray: cancel the close and hide the window instead.
			if app.GetConfig().CloseToTray {
				runtime.WindowHide(ctx)
				return true
			}
			return false
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
	startupLog("exited")
}
