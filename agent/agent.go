package agent

import (
	"log"
	"time"

	"github.com/coreos/coreinit/job"
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
	events     *registry.EventStream
	Manager    *unit.SystemdManager
	Machine    *machine.Machine
	ServiceTTL string
}

func New(registry *registry.Registry, events *registry.EventStream, machine *machine.Machine, ttl string) *Agent {
	mgr := unit.NewSystemdManager(machine)

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, events, mgr, machine, ttl}

	return agent
}

func (a *Agent) Run() {
	a.StartServiceHeartbeatThread()
	a.StartMachineHeartbeatThread()
	a.startEventListeners()
}

// Keep the local statistics in the Registry up to date
func (a *Agent) StartMachineHeartbeatThread() {
	ttl := parseDuration(DefaultMachineTTL)

	heartbeat := func() {
		addrs := a.Machine.GetAddresses()
		a.Registry.SetMachineAddrs(a.Machine, addrs, ttl)
	}

	loop := func() {
		interval := intervalFromTTL(DefaultMachineTTL)
		c := time.Tick(interval)
		for _ = range c {
			log.Printf("MachineHeartbeat")
			heartbeat()
		}
	}

	go loop()
}

// Keep the state of local units in the Registry up to date
func (a *Agent) StartServiceHeartbeatThread() {
	heartbeat := func() {
		localJobs := a.Manager.GetJobs()
		ttl := parseDuration(a.ServiceTTL)
		for _, j := range localJobs {
			if scheduledJob := a.Registry.GetMachineJob(j.Name, a.Machine); scheduledJob != nil {
				log.Printf("Reporting state of Job(%s)", j.Name)
                a.Registry.SaveJobState(&j, ttl)
            } else {
                log.Printf("Local Job(%s) does not appear to be scheduled to this Machine(%s), stopping it", j.Name, a.Machine.BootId)
                a.Manager.StopJob(&j)
            }
		}
	}

	loop := func() {
		interval := intervalFromTTL(a.ServiceTTL)
		c := time.Tick(interval)
		for _ = range c {
			log.Printf("ServiceHeartbeat")
			heartbeat()
		}
	}

	go loop()
}

func (a *Agent) startEventListeners() {
	eventchan := make(chan registry.Event)
	a.events.RegisterJobEventListener(eventchan, a.Machine)

	handlers := map[int]func(registry.Event){
		registry.EventJobCreated: a.handleEventJobCreated,
		registry.EventJobDeleted: a.handleEventJobDeleted,
	}

	for true {
		event := <-eventchan
		log.Printf("Event received: Type=%d", event.Type)

		log.Printf("Event handler begin")
		handlers[event.Type](event)
		log.Printf("Event handler complete")
	}
}

func (a *Agent) handleEventJobCreated(event registry.Event) {
	j := event.Payload.(job.Job)
	log.Printf("EventJobCreated(%s): starting job", j.Name)
	a.Manager.StartJob(&j)
}

func (a *Agent) handleEventJobDeleted(event registry.Event) {
	j := event.Payload.(job.Job)
	log.Printf("EventJobDeleted(%s): stopping job", j.Name)
	a.Manager.StopJob(&j)
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
