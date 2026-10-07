package main

import (
	"errors"
	"time"
)

// minOfDay returns minutes since midnight for a time.
func minOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

// parseClock parses "HH:MM" into minutes since midnight.
func parseClock(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

func fixedDesired(cfg Config, now time.Time) (string, error) {
	d := parseClock(cfg.DarkStart)
	l := parseClock(cfg.LightStart)
	if d < 0 || l < 0 {
		return "", errors.New("invalid fixed times")
	}
	cur := minOfDay(now)
	if d == l {
		return "", errors.New("dark and light start are the same")
	}
	if d < l {
		// dark during the day span [d, l)
		if cur >= d && cur < l {
			return "dark", nil
		}
		return "light", nil
	}
	// crosses midnight: dark overnight
	if cur >= d || cur < l {
		return "dark", nil
	}
	return "light", nil
}

func solarDesired(sunrise, sunset time.Time, now time.Time) (string, error) {
	if sunrise.IsZero() || sunset.IsZero() {
		return "dark", nil // polar day/night fallback
	}
	cur := minOfDay(now)
	sr := minOfDay(sunrise)
	ss := minOfDay(sunset)
	if sr <= ss {
		if cur >= sr && cur < ss {
			return "light", nil
		}
		return "dark", nil
	}
	return "dark", nil
}

// desiredTheme computes whether the current time should be dark or light.
func (a *App) desiredTheme(now time.Time) (string, error) {
	cfg := a.cfg
	switch cfg.Mode {
	case ModeFixed:
		return fixedDesired(cfg, now)
	case ModeSolar:
		loc := a.currentLocation()
		if loc.Error != "" {
			return "", errors.New(loc.Error)
		}
		sr, ss := sunTimes(loc.Lat, loc.Lon, now)
		return solarDesired(sr, ss, now)
	}
	return "", nil
}

// nextTransition returns the next scheduled theme change after now.
func (a *App) nextTransition(now time.Time) time.Time {
	cfg := a.cfg
	if cfg.Mode == ModeSolar {
		loc := a.currentLocation()
		if loc.Error == "" {
			var candidates []time.Time
			for _, d := range []time.Time{now, now.AddDate(0, 0, 1)} {
				sr, ss := sunTimes(loc.Lat, loc.Lon, d)
				candidates = append(candidates, sr, ss)
			}
			return earliestFuture(candidates, now)
		}
	}
	// fixed mode
	var candidates []time.Time
	for _, d := range []time.Time{now, now.AddDate(0, 0, 1)} {
		base := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		if h, m := parseClockHMS(cfg.DarkStart); h >= 0 {
			candidates = append(candidates, base.Add(time.Duration(h)*time.Hour+time.Duration(m)*time.Minute))
		}
		if h, m := parseClockHMS(cfg.LightStart); h >= 0 {
			candidates = append(candidates, base.Add(time.Duration(h)*time.Hour+time.Duration(m)*time.Minute))
		}
	}
	return earliestFuture(candidates, now)
}

func parseClockHMS(s string) (int, int) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return -1, -1
	}
	return t.Hour(), t.Minute()
}

func earliestFuture(times []time.Time, now time.Time) time.Time {
	var best time.Time
	for _, t := range times {
		if t.After(now) && (best.IsZero() || t.Before(best)) {
			best = t
		}
	}
	return best
}
