package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// "About" section bindings (v2.0.0): the app is published on GitHub, so the
// UI can point people at the repository, the releases and the diagnostic log
// folder instead of asking them to find those paths by hand.

const (
	repoURL     = "https://github.com/mathangler/auto-dark-mode"
	releasesURL = repoURL + "/releases/latest"
)

// OpenRepository opens the project page in the default browser.
func (a *App) OpenRepository() {
	if a.ctx == nil {
		return
	}
	runtime.BrowserOpenURL(a.ctx, repoURL)
}

// OpenReleases opens the latest release page (downloads / update check).
func (a *App) OpenReleases() {
	if a.ctx == nil {
		return
	}
	runtime.BrowserOpenURL(a.ctx, releasesURL)
}

// OpenLogsFolder reveals %APPDATA%\AutoDarkMode (app.log lives there).
func (a *App) OpenLogsFolder() {
	dir := filepath.Join(os.Getenv("APPDATA"), "AutoDarkMode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		diagLog("ui: cannot create log folder %s: %v", dir, err)
		return
	}
	// explorer.exe returns a non-zero exit code even on success, so the
	// error is intentionally ignored.
	_ = exec.Command("explorer.exe", dir).Start()
	diagLog("ui: opened log folder %s", dir)
}
