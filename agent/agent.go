package agent

import (
	"strings"
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
	peers map[string][]string
}

func New(registry *registry.Registry, events *registry.EventStream, machine *machine.Machine, ttl string) *Agent {
	mgr := unit.NewSystemdManager(machine)

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, events, mgr, machine, ttl, make(chan bool), make(map[string][]string, 0)}

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

// Determine whether a given Job conflicts with any locally-scheduled Jobs
func (a *Agent) HasLocalJobConflict(j *job.Job) bool {
	isSingleton := func (j *job.Job) bool {
		singleton, ok := j.Payload.Requirements["MachineSingleton"]
		return ok && singleton[0] == "true"
	}

	hasProvides := func (j *job.Job) bool {
		provides, ok := j.Payload.Requirements["Provides"]
		return ok && len(provides) > 0
	}

	if !isSingleton(j) || !hasProvides(j) {
		return false
	}

	for _, other := range a.Registry.GetAllJobsByMachine(a.Machine) {
		if !hasProvides(&other) {
			continue
		}

		// Skip self
		if other.Name == j.Name {
			continue
		}

		for _, provide := range j.Payload.Requirements["Provides"] {
			for _, otherProvide := range other.Payload.Requirements["Provides"] {
				if provide == otherProvide {
					log.V(1).Infof("Local Job(%s) already provides '%s'", other.Name, provide)
					return true
				}
			}
		}
	}

	return false
}

func (a *Agent) HandleEventJobOffered(event registry.Event) {
	jo := event.Payload.(job.JobOffer)
	log.V(1).Infof("EventJobOffered(%s): verifying ability to run Job", jo.Job.Name)

	metadata := extractMachineMetadata(jo.Job.Payload.Requirements)
	if !a.Machine.HasMetadata(metadata) {
		log.V(1).Infof("EventJobOffered(%s): local Metadata insufficient", jo.Job.Name)
		return
	}

	if a.HasLocalJobConflict(&jo.Job) {
		log.V(1).Infof("EventJobOffered(%s): local Job conflict", jo.Job.Name)
		return
	}

	var missing []string
	for _, peerName := range jo.Job.Payload.Peers {
		log.V(1).Infof("EventJobOffered(%s): looking for target of Peer(%s)", jo.Job.Name, peerName)
		//FIXME: ideally the machine would use its own knowledge rather than calling GetJobTarget
		if tgt := a.Registry.GetJobTarget(peerName); tgt == nil || tgt.BootId != a.Machine.BootId {
			log.V(1).Infof("EventJobOffered(%s): unable to run Job, Peer(%s) not scheduled here", jo.Job.Name, peerName)

			log.V(1).Infof("EventJobOffered(%s): tracking Job until Peer(%s) is scheduled", jo.Job.Name, peerName)
			missing = append(missing, peerName)
		} else {
			log.V(1).Infof("EventJobOffered(%s): Peer(%s) scheduled here", jo.Job.Name, peerName)
		}
	}

	if len(missing) > 0 {
		log.V(1).Infof("EventJobOffered(%s): tracking Job until Peers(%s) are scheduled", jo.Job.Name, strings.Join(missing, ","))
		for _, peerName := range missing {
			if _, ok := a.peers[peerName]; !ok {
				a.peers[peerName] = make([]string, 0)
			}
			a.peers[peerName] = append(a.peers[peerName], jo.Job.Name)
		}
		return
	}

	log.Infof("EventJobOffered(%s): submitting JobBid", jo.Job.Name)
	jb := job.NewBid(jo.Job.Name, a.Machine.BootId)
	a.Registry.SubmitJobBid(jb)
}

func extractMachineMetadata(requirements map[string][]string) map[string][]string {
	metadata := make(map[string][]string)

	for key, values := range requirements {
		if !strings.HasPrefix(key, "Machine-") {
			log.V(2).Infof("Skipping requirement %s, not machine metadata.", key)
			continue
		}

		// Strip off leading 'Machine-'
		key = key[8:]

		if len(values) == 0 {
			log.V(2).Infof("Metadata(%s) requirement provided no values, ignoring.", key)
			continue
		}

		metadata[key] = values
	}

	return metadata
}

func (a *Agent) HandleEventJobScheduled(event registry.Event) {
	jobName := event.Payload.(string)

	if event.Context.BootId == a.Machine.BootId {
		log.V(1).Infof("EventJobScheduled(%s): Job scheduled to this Agent", jobName)

		log.V(1).Infof("EventJobScheduled(%s): Fetching Job from Registry", jobName)
		j := a.Registry.GetJob(jobName)

		if a.HasLocalJobConflict(j) {
			log.V(1).Infof("EventJobScheduled(%s): local Job conflict, cancelling", jobName)
			a.Registry.CancelJob(jobName)
			return
		}

		log.Infof("EventJobScheduled(%s): Starting Job", j.Name)
		a.Manager.StartJob(j)

		if peers, ok := a.peers[jobName]; ok {
			for _, peer := range peers {
				log.V(1).Infof("EventJobScheduled(%s): Found unresolved offer for Peer(%s)", jobName, peer)

				log.Infof("EventJobScheduled(%s): submitting JobBid for Peer(%s)", jobName, peer)
				jb := job.NewBid(peer, a.Machine.BootId)
				a.Registry.SubmitJobBid(jb)
			}
		}
	}
}

func (a *Agent) HandleEventJobCancelled(event registry.Event) {
	jobName := event.Payload.(string)
	log.Infof("EventJobCancelled(%s): stopping Job", jobName)
	j := job.NewJob(jobName, nil, nil)
	a.Manager.StopJob(j)

	peers, ok := a.peers[jobName]
	if ok {
		for _, peer := range peers {
			log.Infof("EventJobCancelled(%s): cancelling Peer(%s) of Job", jobName, peer)
			a.Registry.CancelJob(peer)
		}
	}
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
