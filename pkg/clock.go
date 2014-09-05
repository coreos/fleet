package pkg

import (
	"sync"
	"time"
)

// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested
type Clock interface {
	After(d time.Duration) <-chan time.Time
	Sleep(d time.Duration)
}

// NewRealClock returns a Clock which simply delegates calls to the actual time
// package; it should be used by packges in production.
func NewRealClock() Clock {
	return &realClock{}
}

// NewFakeClock returns a Clock which can be manually ticked through time for
// testing.
func NewFakeClock() Clock {
	return &fakeClock{
		l: sync.RWMutex{},
	}
}

type realClock struct{}

func (rc *realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (rc *realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

type fakeClock struct {
	sleepers []*sleeper
	time     time.Time
	l        sync.RWMutex
}

type sleeper struct {
	until time.Time
	done  chan time.Time
}

// After mimics time.After; it waits for the given duration to elapse on the
// fakeClock, then sends the current time on the returned channel.
func (fc *fakeClock) After(d time.Duration) <-chan time.Time {
	done := make(chan time.Time, 1)
	if d.Nanoseconds() == 0 {
		// special case - trigger immediately
		go func() {
			done <- fc.time
		}()
	} else {
		// otherwise, add to the set of sleepers
		end := fc.time.Add(d)
		s := &sleeper{
			until: end,
			done:  done,
		}
		fc.l.Lock()
		fc.sleepers = append(fc.sleepers, s)
		fc.l.Unlock()
	}
	return done
}

// Sleep blocks until the given duration has passed on the fakeClock
func (fc *fakeClock) Sleep(d time.Duration) {
	<-fc.After(d)
}

// Tick advances fakeClock to a new point in time, ensuring channels from any
// previous invocations of After are notified appropriately before returning
func (fc *fakeClock) Tick(d time.Duration) {
	fc.l.Lock()
	end := fc.time.Add(d)
	var newSleepers []*sleeper
	for _, s := range fc.sleepers {
		if end.Sub(s.until) >= 0 {
			s.done <- end
		} else {
			newSleepers = append(newSleepers, s)
		}
	}
	fc.sleepers = newSleepers
	fc.time = end
	fc.l.Unlock()
}
