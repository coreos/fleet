// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"errors"
	"time"

	"github.com/coreos/fleet/heart"
	"github.com/coreos/fleet/log"
)

var ErrShutdown = errors.New("monitor told to shut down")

func NewMonitor(ttl time.Duration) *Monitor {
	return &Monitor{ttl, ttl / 2}
}

type Monitor struct {
	TTL  time.Duration
	ival time.Duration
}

// Monitor periodically checks the given Heart to make sure it
// beats successfully. If the heartbeat check fails for any
// reason, an error is returned. If the supplied channel is
// closed, Monitor returns ErrShutdown.
func (m *Monitor) Monitor(hrt heart.Heart, sdc <-chan struct{}) error {
	ticker := time.Tick(m.ival)
	for {
		select {
		case <-sdc:
			return ErrShutdown
		case <-ticker:
			if _, err := check(hrt, m.TTL); err != nil {
				return err
			}
		}
	}
}

// check attempts to beat a Heart several times within a timeout, returning the
// log index at which the beat succeeded or an error
func check(hrt heart.Heart, ttl time.Duration) (idx uint64, err error) {
	// time out after a third of the machine presence TTL, attempting
	// the heartbeat up to four times
	timeout := ttl / 3
	interval := timeout / 4

	tchan := time.After(timeout)
	next := time.After(0)
	for idx == 0 {
		select {
		case <-tchan:
			err = errors.New("Monitor timed out before successful heartbeat")
			return
		case <-next:
			idx, err = hrt.Beat(ttl)
			if err != nil {
				log.Debugf("Monitor heartbeat function returned err, retrying in %v: %v", interval, err)
			}

			next = time.After(interval)
		}
	}

	return
}
