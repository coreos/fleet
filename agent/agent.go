package agent

import (
	"time"

	log "github.com/golang/glog"

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
	stop       chan bool
}

func New(registry *registry.Registry, events *registry.EventStream, machine *machine.Machine, ttl string) *Agent {
	mgr := unit.NewSystemdManager(machine)

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, events, mgr, machine, ttl, make(chan bool)}

	return agent
}

// Trigger all async processes the Agent intends to run
func (a *Agent) Run() {
	// Kick off the three threads we need for our async processes
	svcstop := a.StartServiceHeartbeatThread()
	machstop := a.StartMachineHeartbeatThread()
	eventstop := a.startEventListeners()

	// Block until we receive a stop signal
	<-a.stop

	// Signal each of the threads we started to also stop
	svcstop <- true
	machstop <- true
	eventstop <- true
}

// Stop all async processes the Agent is running
func (a *Agent) Stop() {
	a.stop <- true
}

// Keep the local statistics in the Registry up to date
func (a *Agent) StartMachineHeartbeatThread() chan bool {
	stop := make(chan bool)
	ttl := parseDuration(DefaultMachineTTL)

	heartbeat := func() {
		addrs := a.Machine.GetAddresses()
		a.Registry.SetMachineAddrs(a.Machine, addrs, ttl)
	}

	loop := func() {
		interval := intervalFromTTL(DefaultMachineTTL)
		c := time.Tick(interval)
		for _ = range c {
			log.V(1).Info("MachineHeartbeat tick")
			select {
			case <-stop:
				log.V(1).Info("MachineHeartbeat exiting due to stop signal")
				return
			default:
				log.V(1).Info("MachineHeartbeat running")
				heartbeat()
			}
		}
	}

	go loop()
	return stop
}

// Keep the state of local units in the Registry up to date
func (a *Agent) StartServiceHeartbeatThread() chan bool {
	stop := make(chan bool)

	heartbeat := func() {
		localJobs := a.Manager.GetJobs()
		ttl := parseDuration(a.ServiceTTL)
		for _, j := range localJobs {
			if scheduledJob := a.Registry.GetMachineJob(j.Name, a.Machine); scheduledJob != nil {
				log.V(1).Infof("Reporting state of Job(%s)", j.Name)
				a.Registry.SaveJobState(&j, ttl)
			} else {
				log.Infof("Local Job(%s) does not appear to be scheduled to this Machine(%s), stopping it", j.Name, a.Machine.BootId)
				a.Manager.StopJob(&j)
			}
		}
	}

	loop := func() {
		interval := intervalFromTTL(a.ServiceTTL)
		c := time.Tick(interval)
		for _ = range c {
			log.V(1).Info("ServiceHeartbeat tick")
			select {
			case <-stop:
				log.V(1).Info("ServiceHeartbeat exiting due to stop signal")
				return
			default:
				log.V(1).Info("ServiceHeartbeat running")
				heartbeat()
			}
		}
	}

	go loop()
	return stop
}

func (a *Agent) startEventListeners() chan bool {
	stop := make(chan bool)

	eventchan := make(chan registry.Event)
	a.events.RegisterJobEventListener(eventchan, a.Machine)

	handlers := map[int]func(registry.Event){
		registry.EventJobCreated: a.handleEventJobCreated,
		registry.EventJobDeleted: a.handleEventJobDeleted,
	}

	loop := func() {
		for true {
			select {
			case <-stop:
				log.V(1).Infof("Exiting event listener loop")
				return
			case event := <-eventchan:
				log.V(1).Infof("Event received: Type=%d", event.Type)

				log.V(1).Infof("Event handler begin")
				handlers[event.Type](event)
				log.V(1).Infof("Event handler complete")
			}
		}
	}

	go loop()
	return stop
}

func (a *Agent) handleEventJobCreated(event registry.Event) {
	j := event.Payload.(job.Job)
	log.Infof("EventJobCreated(%s): starting job", j.Name)
	a.Manager.StartJob(&j)
}

func (a *Agent) handleEventJobDeleted(event registry.Event) {
	j := event.Payload.(job.Job)
	log.Infof("EventJobDeleted(%s): stopping job", j.Name)
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
