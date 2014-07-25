package agent

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
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

func New(mgr unit.UnitManager, uGen *unit.UnitStateGenerator, reg registry.Registry, mach machine.Machine, ttl string) (*Agent, error) {
	ttldur, err := time.ParseDuration(ttl)
	if err != nil {
		return nil, err
	}

	a := &Agent{reg, mgr, uGen, mach, ttldur, &agentCache{}}
	return a, nil
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
func (a *Agent) Heartbeat(stop chan bool) {
	a.heartbeatJobs(a.ttl, stop)
}

func (a *Agent) heartbeatJobs(ttl time.Duration, stop chan bool) {
	heartbeat := func() {
		machID := a.Machine.State().ID
		launched := a.cache.launchedJobs()
		for _, j := range launched {
			go a.registry.JobHeartbeat(j, machID, ttl)
		}
	}

	interval := ttl / 2
	ticker := time.Tick(interval)
	for {
		select {
		case <-stop:
			log.V(1).Info("HeartbeatJobs exiting due to stop signal")
			return
		case <-ticker:
			log.V(1).Info("HeartbeatJobs tick")
			heartbeat()
		}
	}
}

func (a *Agent) loadJob(j *job.Job) error {
	a.cache.setTargetState(j.Name, job.JobStateLoaded)
	a.uGen.Subscribe(j.Name)
	return a.um.Load(j.Name, j.Unit)
}

func (a *Agent) unloadJob(jobName string) {
	go func() {
		a.um.Stop(jobName)
		a.uGen.Unsubscribe(jobName)
		a.um.Unload(jobName)
	}()

	a.registry.ClearJobHeartbeat(jobName)
	a.registry.RemoveUnitState(jobName)
	a.cache.dropTargetState(jobName)
}

func (a *Agent) startJob(jobName string) {
	a.cache.setTargetState(jobName, job.JobStateLaunched)

	machID := a.Machine.State().ID
	a.registry.JobHeartbeat(jobName, machID, a.ttl)

	go func() {
		a.um.Start(jobName)
	}()
}

func (a *Agent) stopJob(jobName string) {
	a.cache.setTargetState(jobName, job.JobStateLoaded)
	a.registry.ClearJobHeartbeat(jobName)

	go func() {
		a.um.Stop(jobName)
	}()
}

// jobs returns a collection of all Jobs that the Agent has either loaded
// or launched. The Unit, TargetState and TargetMachineID fields of the
// returned *job.Job objects are not properly hydrated.
func (a *Agent) jobs() (map[string]*job.Job, error) {
	launched := pkg.NewUnsafeSet()
	for _, jName := range a.cache.launchedJobs() {
		launched.Add(jName)
	}

	units, err := a.um.Units()
	if err != nil {
		return nil, fmt.Errorf("failed fetching loaded units from UnitManager: %v", err)
	}

	filter := pkg.NewUnsafeSet()
	for _, u := range units {
		filter.Add(u)
	}
	states, err := a.um.GetUnitStates(filter)
	if err != nil {
		return nil, fmt.Errorf("failed fetching unit states: %v", err)
	}

	jobs := make(map[string]*job.Job)
	for _, uName := range units {
		jobs[uName] = &job.Job{
			Name:      uName,
			UnitState: states[uName],
			State:     nil,

			// The following fields are not properly populated
			// and should not be used in the calling code
			Unit:            unit.Unit{},
			TargetState:     job.JobState(""),
			TargetMachineID: "",
		}

		js := job.JobStateLoaded
		if launched.Contains(uName) {
			js = job.JobStateLaunched
		}
		jobs[uName].State = &js
	}

	return jobs, nil
}
