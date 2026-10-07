package main

import (
	"math"
	"time"
)

const (
	deg2rad = math.Pi / 180.0
	rad2deg = 180.0 / math.Pi
	// zenith for official sunrise/sunset (90°50')
	zenith = 90.833333
)

func mod360(v float64) float64 {
	v = math.Mod(v, 360)
	if v < 0 {
		v += 360
	}
	return v
}

func mod24(v float64) float64 {
	v = math.Mod(v, 24)
	if v < 0 {
		v += 24
	}
	return v
}

// solarHourUTC computes the UTC hour (0..24) of sunrise (isRise=true) or
// sunset on day-of-year n at the given location. The second return value is
// false when the sun never rises/sets that day (polar region).
//
// Implements the NOAA "sunrise equation" (Spencer/Almanac) algorithm.
func solarHourUTC(lat, lon, lonHour, n float64, isRise bool) (float64, bool) {
	// Approximate time of the event in fractional day-of-year.
	t := n
	if isRise {
		t += (6.0 - lonHour) / 24.0
	} else {
		t += (18.0 - lonHour) / 24.0
	}

	m := (0.9856*t - 3.289) * deg2rad // solar mean anomaly (rad)
	// True (ecliptic) longitude of the sun, normalized to 0..360.
	l0 := mod360(0.9856*t - 3.289 + 1.916*math.Sin(m) + 0.020*math.Sin(2*m) + 282.634)
	l0rad := l0 * deg2rad

	// Right ascension, corrected to the same quadrant as the longitude.
	ra := math.Atan(0.91764*math.Tan(l0rad)) * rad2deg
	ra = mod360(ra + (math.Floor(l0/90)-math.Floor(ra/90))*90)

	// Declination.
	sinDec := 0.39782 * math.Sin(l0rad)
	cosDec := math.Cos(math.Asin(sinDec))

	latRad := lat * deg2rad
	cosH := (math.Cos(zenith*deg2rad) - sinDec*math.Sin(latRad)) / (cosDec * math.Cos(latRad))
	if cosH < -1 || cosH > 1 {
		return 0, false // sun stays above/below horizon all day
	}

	var hDeg float64
	if isRise {
		hDeg = 360 - math.Acos(cosH)*rad2deg
	} else {
		hDeg = math.Acos(cosH) * rad2deg
	}

	// Local mean time (hours), then convert to UT.
	tHours := hDeg/15.0 + ra/15.0 - 0.06571*t - 6.622
	return mod24(tHours - lonHour), true
}

// sunTimes returns the sunrise and sunset local times for `date` at the
// given coordinates, expressed in the same location/zone as `date`.
func sunTimes(lat, lon float64, date time.Time) (sunrise, sunset time.Time) {
	if lat < -90 || lat > 90 {
		return sunrise, sunset
	}
	loc := date.Location()
	_, offsetSec := date.Zone()
	offsetHours := float64(offsetSec) / 3600.0
	base := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	n := float64(date.YearDay())
	lonHour := lon / 15.0

	if riseUTC, ok := solarHourUTC(lat, lon, lonHour, n, true); ok {
		sunrise = base.Add(time.Duration(mod24(riseUTC+offsetHours) * float64(time.Hour)))
	}
	if setUTC, ok := solarHourUTC(lat, lon, lonHour, n, false); ok {
		sunset = base.Add(time.Duration(mod24(setUTC+offsetHours) * float64(time.Hour)))
	}
	return sunrise, sunset
}
