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

// Job is one user's scheduled bio touch: ChatID identifies where to send the
// outcome notification, matching that user's Telegram user ID.
type Job struct {
	ChatID int64
	Touch  func() error
}

// Run blocks forever, invoking every job in turn (one by one — never in
// parallel, to keep at most one browser alive at a time) at each of the
// day's scheduled times, reporting each job's outcome via notify. Intended
// to be started in its own goroutine for the life of the process.
func Run(jobs []Job, notify func(chatID int64, text string)) {
	for {
		next := nextRun(time.Now().In(location))
		time.Sleep(time.Until(next))

		log.Println("scheduler: running scheduled bio touch")
		for _, j := range jobs {
			if err := j.Touch(); err != nil {
				log.Printf("scheduler: job for chat %d failed: %v", j.ChatID, err)
				notify(j.ChatID, fmt.Sprintf("Scheduled bio touch failed: %v", err))
				continue
			}
			log.Printf("scheduler: job for chat %d succeeded", j.ChatID)
			notify(j.ChatID, "Scheduled bio touch: success.")
		}
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
