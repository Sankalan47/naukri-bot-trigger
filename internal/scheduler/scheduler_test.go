package scheduler

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	loc := time.UTC
	day := func(hour, min int) time.Time {
		return time.Date(2026, 7, 4, hour, min, 0, 0, loc)
	}

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before first slot", day(6, 0), day(9, 15)},
		{"between first and second slot", day(10, 0), day(14, 30)},
		{"between second and third slot", day(15, 0), day(17, 0)},
		{"after last slot rolls to tomorrow", day(18, 0), time.Date(2026, 7, 5, 9, 15, 0, 0, loc)},
		{"exactly on a slot is not chosen again", day(9, 15), day(14, 30)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextRun(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("nextRun(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}
