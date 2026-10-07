package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
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

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Auto Dark Mode",
		Width:     900,
		Height:    640,
		MinWidth:  760,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 26, B: 32, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
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
