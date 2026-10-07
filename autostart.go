package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// exePath returns the full path of the running executable.
func exePath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	return p
}

// startupLog appends a timestamped line to a log under the app's config
// directory, so boot behaviour (e.g. auto-start) can be verified after login.
func startupLog(msg string) {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	p := filepath.Join(base, "AutoDarkMode", "startup.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

// setAutoStart registers (or unregisters) the app to launch at login.
func (a *App) setAutoStart(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// When run via `wails dev` the executable is a helper binary in a temp
	// dir; only register the real installed binary.
	if filepath.Base(exe) == "wails.exe" {
		return nil
	}
	name := "AutoDarkMode"
	if !enabled {
		k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, `"`+exe+`"`)
}
