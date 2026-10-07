package main

// Version is THE app version: shown in the settings footer, logged at
// startup, and (via wails.json info.productVersion) stamped into the exe
// file properties and the NSIS installer's DisplayVersion.
//
// Do not edit by hand — bump with build\bump-version.ps1 -Patch|-Minor|-Major,
// which syncs every version location at once. The rules, file inventory and
// version history live in docs/VERSIONING.md.
const Version = "1.3.0"

// GetVersion exposes the version to the settings UI (Wails binding).
func (a *App) GetVersion() string {
	return Version
}
