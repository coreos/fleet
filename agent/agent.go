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

	// map of Peer dependencies to unresolved JobOffers
	peers map[string]string
}

func New(registry *registry.Registry, events *registry.EventStream, machine *machine.Machine, ttl string) *Agent {
	mgr := unit.NewSystemdManager(machine)

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, events, mgr, machine, ttl, make(chan bool), make(map[string]string, 0)}

	return agent
}

// Trigger all async processes the Agent intends to run
func (a *Agent) Run() {
	// Kick off the three threads we need for our async processes
	svcstop := a.StartServiceHeartbeatThread()
	machstop := a.StartMachineHeartbeatThread()

	a.events.AddListener("agent", a.Machine, a)

	// Block until we receive a stop signal
	<-a.stop

	// Signal each of the threads we started to also stop
	svcstop <- true
	machstop <- true

	a.events.RemoveListener("agent", a.Machine)
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
		a.Registry.SetMachineState(a.Machine, ttl)
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
			if tgt := a.Registry.GetJobTarget(j.Name); tgt != nil && tgt.BootId == a.Machine.BootId {
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

func (a *Agent) HandleEventJobOffered(event registry.Event) {
	jo := event.Payload.(job.JobOffer)
	log.V(1).Infof("EventJobOffered(%s): verifying ability to run Job", jo.Job.Name)

	for _, peerName := range jo.Job.Peers {
		//FIXME: ideally the machine would use its own knowledge rather than calling GetJobTarget
		if tgt := a.Registry.GetJobTarget(peerName); tgt == nil || tgt.BootId != a.Machine.BootId {
			log.V(1).Infof("EventJobOffered(%s): unable to run Job, Peer(%s) not scheduled here", jo.Job.Name, peerName)

			log.V(1).Infof("EventJobOffered(%s): tracking Job until Peer(%s) is scheduled", jo.Job.Name, peerName)
			a.peers[peerName] = jo.Job.Name

			return
		}
	}

	for key, vals := range jo.Job.Requirements {
		if len(vals) == 0 {
			log.V(2).Infof("EventJobOffered(%s): required Metadata(%s) provided no values, skipping assertion.", jo.Job.Name, key)
			continue
		}

		local, ok := a.Machine.Metadata[key]
		if !ok {
			log.V(1).Infof("EventJobOffered(%s): no local values found for required Metadata(%s), unable to run job", jo.Job.Name, key)
			return
		}

		log.V(2).Infof("EventJobOffered(%s): asserting local Metadata(%s) meets requirements", jo.Job.Name, key)

		var localMatch bool
		for _, val := range vals {
			if local == val {
				log.V(1).Infof("EventJobOffered(%s): local Metadata(%s) meets requirement", jo.Job.Name, key)
				localMatch = true
			}
		}

		if !localMatch {
			log.V(1).Infof("EventJobOffered(%s): local Metadata(%s) does not match requirement, unable to run job", jo.Job.Name, key)
			return
		}
	}

	log.Infof("EventJobOffered(%s): submitting JobBid", jo.Job.Name)
	jb := job.NewBid(jo.Job.Name, a.Machine.BootId)
	a.Registry.SubmitJobBid(jb)
}

func (a *Agent) HandleEventJobScheduled(event registry.Event) {
	jobName := event.Payload.(string)

	if event.Context.BootId == a.Machine.BootId {
		a.handleEventJobScheduledLocally(jobName)
	} else {
		a.handleEventJobScheduledElsewhere(jobName)
	}
}

func (a *Agent) handleEventJobScheduledLocally(jobName string) {
	log.V(1).Infof("EventJobScheduled(%s): Job scheduled to this Agent", jobName)

	log.V(1).Infof("EventJobScheduled(%s): Fetching Job from Registry", jobName)
	j := a.Registry.GetJob(jobName)

	log.Infof("EventJobScheduled(%s): Starting Job", j.Name)
	a.Manager.StartJob(j)

	if peerName, ok := a.peers[jobName]; ok {
		log.V(1).Infof("EventJobScheduled(%s): Found unresolved offer for Peer(%s)", jobName, peerName)

		log.V(1).Infof("EventJobScheduled(%s): Removing Job from local peer list", jobName)
		delete(a.peers, jobName)

		log.Infof("EventJobScheduled(%s): submitting JobBid for Peer(%s)", jobName, peerName)
		jb := job.NewBid(peerName, a.Machine.BootId)
		a.Registry.SubmitJobBid(jb)
	}
}

func (a *Agent) handleEventJobScheduledElsewhere(jobName string) {
	log.V(1).Infof("EventJobScheduled(%s): Job not scheduled to local Agent", jobName)

	if _, ok := a.peers[jobName]; ok {
		log.V(1).Infof("EventJobScheduled(%s): Removing Job from local peer list", jobName)
		delete(a.peers, jobName)

		//TODO: Also remove any keys where value=jobName - this is just extra cleanup
	}
}

func (a *Agent) HandleEventJobCancelled(event registry.Event) {
	jobName := event.Payload.(string)
	log.Infof("EventJobCancelled(%s): stopping job", jobName)
	j, _ := job.NewJob(jobName, nil, nil, make(map[string][]string, 0))
	a.Manager.StopJob(j)
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
