package heartbeat

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

func NewHeart(beat func() error, interval time.Duration) *Heart {
	return &Heart{beat, interval}
}

// Heart is used to heartbeat a function at a specified interval
type Heart struct {
	beat func() error
	ival time.Duration
}

// Start triggers a Heart's periodic beating. It will continue
// attempting to beat until the provided channel is closed. The
// beat function is tried up to three times within each period.
func (h *Heart) Start(stop chan bool) {
	attempts := 3
	ticker := time.Tick(h.ival)
	for {
		select {
		case <-stop:
			log.V(1).Info("Heartbeat exiting due to stop signal")
			return
		case <-ticker:
			log.V(1).Info("Heartbeat tick")
			if err := attempt(attempts, h.beat); err != nil {
				log.Errorf("Failed heartbeat after %d attempts: %v", attempts, err)
			}
		}
	}
}

// attempt runs the provided function up to N times. It checks for a
// nil error value each time, halting operation if it is found.
// If a nil error value is not found, the error received from the
// final attempt will be returned.
func attempt(attempts int, fn func() error) (err error) {
	if attempts < 1 {
		return fmt.Errorf("attempts argument must be 1 or greater, got %d", attempts)
	}

	// The amount of time the retry mechanism waits after a failed attempt
	// doubles following each failure. This is a simple exponential backoff.
	sleep := time.Second

	for i := 1; i <= attempts; i++ {
		err = fn()
		if err == nil || i == attempts {
			break
		}

		sleep = sleep * 2
		log.V(1).Infof("function returned err, retrying in %v: %v", sleep, err)
		time.Sleep(sleep)
	}

	return err
}
