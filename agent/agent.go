package agent

import (
	"log"
	"time"

	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
	"github.com/coreos/coreinit/unit"
)

const (
	DefaultServiceTTL = "2s"
	DefaultMachineTTL = "10s"
	refreshInterval   = 2 // Refresh TTLs at 1/2 the TTL length
)

// The Agent owns all of the coordination between the Registry, the local
// Machine, and the local SystemdManager.
type Agent struct {
	Registry   *registry.Registry
	Manager    *unit.SystemdManager
	Machine    *machine.Machine
	ServiceTTL string
}

func (a *Agent) DoHeartbeat() {
	go a.doServiceHeartbeat()
	a.doMachineHeartbeat()
	return
}

// Keep the local statistics in the Registry up to date
func (a *Agent) doMachineHeartbeat() {
	interval := intervalFromTTL(DefaultMachineTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick machine heartbeat")
		a.UpdateMachine()
	}
}

func (a *Agent) UpdateMachine() {
	addrs := a.Machine.GetAddresses()
	ttl := parseDuration(DefaultMachineTTL)
	log.Println("Updating machine", a.Machine, "with addrs", addrs)
	a.Registry.SetMachineAddrs(a.Machine, addrs, ttl)
}

// Keep the state of local units in the Registry up to date
func (a *Agent) doServiceHeartbeat() {
	interval := intervalFromTTL(a.ServiceTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick service heartbeat")
		a.UpdateJobs()
	}
}

func (a *Agent) UpdateJobs() {
	registeredJobs := a.Registry.GetMachineJobs(a.Machine)
	localJobs := a.Manager.GetJobs()

	for _, job := range registeredJobs {
		_, ok := localJobs[job.Name]
		if !ok {
			a.Manager.StartJob(&job)
		} else if state := a.Manager.GetJobState(&job); state == nil || state.ActiveState != "active"{
			a.Manager.StartJob(&job)
		}
	}

	// Fetch local jobs again since state may have changed above
	localJobs = a.Manager.GetJobs()

	ttl := uint64(parseDuration(a.ServiceTTL).Seconds())
	for _, job := range localJobs {
		_, ok := registeredJobs[job.Name]

		if ok {
			a.Registry.UpdateJob(&job, ttl)
		} else {
			a.Manager.StopJob(&job)
		}
	}
}

func New(registry *registry.Registry, machine *machine.Machine, ttl string) *Agent {
	mgr := unit.NewSystemdManager(machine)

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, mgr, machine, ttl}

	return agent
}

func parseDuration(d string) time.Duration {
	duration, err := time.ParseDuration(d)
	if err != nil {
		panic(err)
	}

	return duration
}

func intervalFromTTL(ttl string) time.Duration {
	duration := parseDuration(ttl)
	return duration / refreshInterval
}
