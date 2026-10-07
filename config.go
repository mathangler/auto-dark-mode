package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Mode selects how the app decides when to switch themes.
type Mode string

const (
	ModeFixed Mode = "fixed" // user-supplied times
	ModeSolar Mode = "solar" // sunrise/sunset from location
)

// LocationSource is how the app obtains its coordinates.
type LocationSource string

const (
	SourceAuto   LocationSource = "auto"   // GPS, then IP geolocation
	SourceManual LocationSource = "manual" // lat/lon typed by the user
	SourceCity   LocationSource = "city"   // picked from the built-in table
)

// ManualTheme holds a user override. "auto" means follow the schedule.
type ManualTheme string

const (
	ThemeAuto  ManualTheme = "auto"
	ThemeDark  ManualTheme = "dark"
	ThemeLight ManualTheme = "light"
)

// Config is the persisted application settings.
type Config struct {
	Enabled      bool           `json:"enabled"`
	Mode         Mode           `json:"mode"`
	DarkStart    string         `json:"darkStart,omitempty"`  // "HH:MM", fixed mode
	LightStart   string         `json:"lightStart,omitempty"` // "HH:MM", fixed mode

	LocationSource LocationSource `json:"locationSource"`
	Lat            float64        `json:"lat,omitempty"`
	Lon            float64        `json:"lon,omitempty"`
	City           string         `json:"city,omitempty"`

	AutoStart  bool       `json:"autoStart"`
	ManualTheme ManualTheme `json:"manualTheme"`

	// Lang is "en", "zh", or "" to follow the system UI language.
	Lang string `json:"lang"`
	// CloseToTray hides the window to the tray instead of quitting on close.
	CloseToTray bool `json:"closeToTray"`

	// Resolved location cache (not persisted directly; recalc each session).
	resolvedLat     float64
	resolvedLon     float64
	resolvedSource  string
	resolvedName    string
	lastTransition  time.Time
}

func defaultConfig() Config {
	return Config{
		Enabled:        true,
		Mode:           ModeSolar,
		DarkStart:      "19:00",
		LightStart:     "07:00",
		LocationSource: SourceAuto,
		ManualTheme:    ThemeAuto,
		CloseToTray:    true,
	}
}

func configPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "AutoDarkMode", "config.json")
}

func loadConfig() Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	// merge over defaults so new fields get their default values
	var tmp = defaultConfig()
	if err := json.Unmarshal(data, &tmp); err != nil {
		return cfg
	}
	if tmp.DarkStart == "" {
		tmp.DarkStart = cfg.DarkStart
	}
	if tmp.LightStart == "" {
		tmp.LightStart = cfg.LightStart
	}
	if tmp.ManualTheme == "" {
		tmp.ManualTheme = ThemeAuto
	}
	if tmp.LocationSource == "" {
		tmp.LocationSource = SourceAuto
	}
	if tmp.Mode == "" {
		tmp.Mode = ModeSolar
	}
	return tmp
}

func (c *Config) save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}
