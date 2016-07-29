// Copyright 2014 The fleet Authors
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

package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

const (
	// TTL to use with all state pushed to Registry
	DefaultTTL = "30s"
)

type Agent struct {
	registry registry.Registry
	um       unit.UnitManager
	uGen     *unit.UnitStateGenerator
	Machine  machine.Machine
	ttl      time.Duration

	cache *agentCache
}

func New(mgr unit.UnitManager, uGen *unit.UnitStateGenerator, reg registry.Registry, mach machine.Machine, ttl time.Duration) *Agent {
	return &Agent{reg, mgr, uGen, mach, ttl, &agentCache{}}
}

func (a *Agent) MarshalJSON() ([]byte, error) {
	data := struct {
		Cache *agentCache
	}{
		Cache: a.cache,
	}
	return json.Marshal(data)
}

// Heartbeat updates the Registry periodically with an acknowledgement of the
// Jobs this Agent is expected to be running.
func (a *Agent) Heartbeat(stop <-chan struct{}) {
	a.heartbeatJobs(a.ttl, stop)
}

func (a *Agent) heartbeatJobs(ttl time.Duration, stop <-chan struct{}) {
	heartbeat := func() {
		machID := a.Machine.State().ID
		launched := a.cache.launchedJobs()
		for _, j := range launched {
			go a.registry.UnitHeartbeat(j, machID, ttl)
		}
	}

	interval := ttl / 2
	ticker := time.Tick(interval)
	for {
		select {
		case <-stop:
			log.Debug("HeartbeatJobs exiting due to stop signal")
			return
		case <-ticker:
			log.Debug("HeartbeatJobs tick")
			heartbeat()
		}
	}
}

func (a *Agent) reloadUnitFiles() error {
	return a.um.ReloadUnitFiles()
}

func (a *Agent) loadUnit(u *job.Unit) error {
	a.cache.setTargetState(u.Name, job.JobStateLoaded)
	a.uGen.Subscribe(u.Name)
	return a.um.Load(u.Name, u.Unit)
}

func (a *Agent) unloadUnit(unitName string) error {
	a.registry.ClearUnitHeartbeat(unitName)
	a.cache.dropTargetState(unitName)

	errStop := a.um.TriggerStop(unitName)
	if errStop != nil {
		log.Warningf("TriggerStop on systemd unit %s returned: %v", unitName, errStop)
	} else {
		log.Infof("Stopped unit(%s)", unitName)
	}

	a.uGen.Unsubscribe(unitName)

	// unit should be unloaded and unit file should be removed, only if the unit
	// could be successfully stopped. Otherwise the unit could get into a state
	// where the unit cannot be stopped via fleet, because the unit file was
	// already removed. See also https://github.com/coreos/fleet/issues/1216.
	var errUnload error
	if errStop == nil {
		errUnload = a.um.Unload(unitName)
	}

	return errUnload
}

func (a *Agent) startUnit(unitName string) error {
	a.cache.setTargetState(unitName, job.JobStateLaunched)

	machID := a.Machine.State().ID
	a.registry.UnitHeartbeat(unitName, machID, a.ttl)

	return a.um.TriggerStart(unitName)
}

func (a *Agent) stopUnit(unitName string) error {
	a.cache.setTargetState(unitName, job.JobStateLoaded)
	a.registry.ClearUnitHeartbeat(unitName)

	return a.um.TriggerStop(unitName)
}

type unitState struct {
	state job.JobState
	hash  string
}
type unitStates map[string]unitState

// units returns a map representing the current state of units known by the agent.
func (a *Agent) units() (unitStates, error) {
	launched := pkg.NewUnsafeSet()
	for _, jName := range a.cache.launchedJobs() {
		launched.Add(jName)
	}

	loaded := pkg.NewUnsafeSet()
	for _, jName := range a.cache.loadedJobs() {
		loaded.Add(jName)
	}

	units, err := a.um.Units()
	if err != nil {
		return nil, fmt.Errorf("failed fetching loaded units from UnitManager: %v", err)
	}

	filter := pkg.NewUnsafeSet()
	for _, u := range units {
		filter.Add(u)
	}

	uStates, err := a.um.GetUnitStates(filter)
	if err != nil {
		return nil, fmt.Errorf("failed fetching unit states from UnitManager: %v", err)
	}

	states := make(unitStates)
	for uName, uState := range uStates {
		js := job.JobStateInactive
		if loaded.Contains(uName) {
			js = job.JobStateLoaded
		} else if launched.Contains(uName) {
			js = job.JobStateLaunched
		}
		us := unitState{
			state: js,
			hash:  uState.UnitHash,
		}
		states[uName] = us
	}

	return states, nil
}
