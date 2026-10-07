package main

import (
	"testing"
	"time"
)

// Known reference values (America/New_York):
//   June 21: sunrise ~05:25 EDT, sunset ~20:31 EDT
//   Dec 21: sunrise ~07:16 EST, sunset ~16:32 EST
func TestSunTimesNewYork(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal("load location:", err)
	}
	lat, lon := 40.7128, -74.0060

	check := func(month time.Month, day int, wantRise, wantSet string) {
		d := time.Date(2024, month, day, 12, 0, 0, 0, loc)
		rise, set := sunTimes(lat, lon, d)
		for _, tc := range []struct {
			label string
			got   time.Time
			want  string
		}{
			{"sunrise", rise, wantRise},
			{"sunset", set, wantSet},
		} {
			w, _ := time.Parse("15:04", tc.want)
			got := tc.got.Hour()*60 + tc.got.Minute()
			exp := w.Hour()*60 + w.Minute()
			if diff := abs(got - exp); diff > 15 {
				t.Errorf("%s on %d/%d: got %s want %s", tc.label, month, day, tc.got.Format("15:04"), tc.want)
			}
		}
	}

	check(time.June, 21, "05:25", "20:31")
	check(time.December, 21, "07:16", "16:32")
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
