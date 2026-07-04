// Package scheduler runs a job at a small set of fixed times every day,
// independent of any Telegram interaction — used to keep the Naukri profile
// looking recently updated even when nobody triggers it manually.
package scheduler

import (
	"fmt"
	"log"
	"time"
)

// runTimes are the times of day (in location) the job runs. Must stay sorted
// ascending for nextRun to work.
var runTimes = []time.Duration{
	9*time.Hour + 15*time.Minute,
	14*time.Hour + 30*time.Minute,
	17 * time.Hour,
}

var location = mustLoadLocation("Asia/Kolkata")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("scheduler: failed to load %s timezone (%v); falling back to server local time", name, err)
		return time.Local
	}
	return loc
}

// Run blocks forever, invoking job once at each of the day's scheduled times
// and reporting the outcome via notify. Intended to be started in its own
// goroutine for the life of the process.
func Run(job func() error, notify func(string)) {
	for {
		next := nextRun(time.Now().In(location))
		time.Sleep(time.Until(next))

		log.Println("scheduler: running scheduled bio touch")
		if err := job(); err != nil {
			log.Printf("scheduler: job failed: %v", err)
			notify(fmt.Sprintf("Scheduled bio touch failed: %v", err))
			continue
		}
		log.Println("scheduler: job succeeded")
		notify("Scheduled bio touch: success.")
	}
}

// nextRun returns the next scheduled instant after now: the earliest of
// today's remaining runTimes, or tomorrow's first slot if none remain today.
func nextRun(now time.Time) time.Time {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, d := range runTimes {
		t := midnight.Add(d)
		if t.After(now) {
			return t
		}
	}
	return midnight.AddDate(0, 0, 1).Add(runTimes[0])
}
