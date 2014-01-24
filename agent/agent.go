package agent

import (
	"fmt"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
	"github.com/coreos/coreinit/unit"
)

const (
	// TTL to use with all state pushed to Registry
	DefaultMachineTTL = "10s"

	// Refresh TTLs at 1/2 the TTL length
	refreshInterval   = 2
)

// The Agent owns all of the coordination between the Registry, the local
// Machine, and the local SystemdManager.
type Agent struct {
	registry   *registry.Registry
	events     *event.EventBus
	machine    *machine.Machine
	ttl        time.Duration

	manager    *unit.SystemdManager
	state      *AgentState

	// channel used to shutdown any open connections/channels the Agent holds
	stop chan bool
}

func New(registry *registry.Registry, events *event.EventBus, machine *machine.Machine, unitPrefix string) *Agent {
	mgr := unit.NewSystemdManager(machine, unitPrefix)
	ttl := parseDuration(DefaultMachineTTL)
	return &Agent{registry, events, machine, ttl, mgr, nil, make(chan bool)}
}

// Trigger all async processes the Agent intends to run
func (a *Agent) Run() {
	a.state = NewState()

	// Kick off the three threads we need for our async processes
	svcstop := a.StartServiceHeartbeatThread()
	machstop := a.StartMachineHeartbeatThread()

	a.events.AddListener("agent", a.machine, a)

	// Block until we receive a stop signal
	<-a.stop

	// Signal each of the threads we started to also stop
	svcstop <- true
	machstop <- true

	a.events.RemoveListener("agent", a.machine)

	a.state = nil
}

// Stop all async processes the Agent is running
func (a *Agent) Stop() {
	a.stop <- true
}

// Keep the local statistics in the Registry up to date
func (a *Agent) StartMachineHeartbeatThread() chan bool {
	stop := make(chan bool)

	heartbeat := func() {
		a.registry.SetMachineState(a.machine, a.ttl)
	}

	loop := func() {
		interval := intervalFromTTL(a.ttl)
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
		localJobs := a.manager.GetJobs()
		for _, j := range localJobs {
			if tgt := a.registry.GetJobTarget(j.Name); tgt != nil && tgt.BootId == a.machine.BootId {
				log.V(1).Infof("Reporting state of Job(%s)", j.Name)
				a.registry.SaveJobState(&j, a.ttl)
			} else {
				log.Infof("Local Job(%s) does not appear to be scheduled to this Machine(%s), stopping it", j.Name, a.machine.BootId)
				a.manager.StopJob(&j)
			}
		}
	}

	loop := func() {
		interval := intervalFromTTL(a.ttl)
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

// Determine whether a given Job conflicts with any other relevant Jobs
// Only call this from a locked AgentState context
func (a *Agent) hasConflict(j *job.Job) bool {
	var reqString string
	for key, slice := range j.Payload.Requirements {
		reqString += fmt.Sprintf("%s = [", key)
		for _, val := range slice {
			reqString += fmt.Sprintf("%s, ", val)
		}
		reqString += fmt.Sprint("] ")
	}

	if len(reqString) > 0 {
		log.V(1).Infof("Job(%s) has requirements %s", j.Name, reqString)
	} else {
		log.V(1).Infof("Job(%s) has no requirements", j.Name)
	}

	isSingleton := func(j *job.Job) bool {
		singleton, ok := j.Payload.Requirements["MachineSingleton"]
		return ok && singleton[0] == "true"
	}

	hasProvides := func(j *job.Job) bool {
		provides, ok := j.Payload.Requirements["Provides"]
		return ok && len(provides) > 0
	}

	if !isSingleton(j) {
		log.V(1).Infof("Job(%s) is not a singleton, therefore no conflict", j.Name)
		return false
	}

	if !hasProvides(j) {
		log.V(1).Infof("Job(%s) does not provide anything, therefore no conflict", j.Name)
		return false
	}

	// Check for conflicts with locally-scheduled jobs
	for _, other := range a.registry.GetAllJobsByMachine(a.machine) {
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

	for _, offer := range a.state.GetBadeOffers() {
		// Skip self
		if offer.Job.Name == j.Name {
			continue
		}

		if !hasProvides(&offer.Job) {
			log.V(1).Infof("Outstanding JobBid(%s) does not provide anything, therefore no conflict", offer.Job.Name)
			continue
		}

		for _, provide := range j.Payload.Requirements["Provides"] {
			for _, offerProvide := range offer.Job.Payload.Requirements["Provides"] {
				if provide == offerProvide {
					log.V(1).Infof("Outstanding JobBid(%s) already provides '%s'", offer.Job.Name, provide)
					return true
				}
			}
		}
	}

	return false
}

func (a *Agent) HandleEventJobOffered(ev event.Event) {
	jo := ev.Payload.(job.JobOffer)
	log.V(1).Infof("EventJobOffered(%s): verifying ability to run Job", jo.Job.Name)

	a.state.Lock()
	defer a.state.Unlock()

	// Everything we check against could change over time, so we track all
	// offers starting here for future bidding even if we can't bid now
	a.state.TrackOffer(jo)
	a.state.TrackJobPeers(jo.Job.Name, jo.Job.Payload.Peers)

	metadata := extractMachineMetadata(jo.Job.Payload.Requirements)
	if !a.machine.HasMetadata(metadata) {
		log.V(1).Infof("EventJobOffered(%s): local Machine Metadata insufficient", jo.Job.Name)
		return
	}

	if a.hasConflict(&jo.Job) {
		log.V(1).Infof("EventJobOffered(%s): local Job conflict, ignoring offer", jo.Job.Name)
		return
	}

	if !a.hasAllLocalPeers(&jo.Job) {
		log.V(1).Infof("EventJobOffered(%s): necessary peer Jobs are not running locally", jo.Job.Name)
		return
	}

	log.Infof("EventJobOffered(%s): passed all criteria, submitting JobBid", jo.Job.Name)
	jb := job.NewBid(jo.Job.Name, a.machine.BootId)
	a.registry.SubmitJobBid(jb)
	a.state.TrackBid(jo.Job.Name)
}

func (a *Agent) hasAllLocalPeers(j *job.Job) bool {
	for _, peerName := range j.Payload.Peers {
		log.V(1).Infof("Looking for target of Peer(%s)", peerName)

		//FIXME: ideally the machine would use its own knowledge rather than calling GetJobTarget
		if tgt := a.registry.GetJobTarget(peerName); tgt == nil || tgt.BootId != a.machine.BootId {
			log.V(1).Infof("Peer(%s) of Job(%s) not scheduled here", peerName, j.Name)
			return false
		} else {
			log.V(1).Infof("Peer(%s) of Job(%s) scheduled here", peerName, j.Name)
		}
	}
	return true
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

func (a *Agent) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)

	a.state.Lock()
	defer a.state.Unlock()

	a.state.DropOffer(jobName)
	a.state.DropBid(jobName)

	if ev.Context.BootId != a.machine.BootId {
		log.V(1).Infof("EventJobScheduled(%s): Job not scheduled to this Agent", jobName)
		a.bidForPossibleOffers()
		return
	} else {
		log.V(1).Infof("EventJobScheduled(%s): Job scheduled to this Agent", jobName)
	}

	log.V(1).Infof("EventJobScheduled(%s): Fetching Job from Registry", jobName)
	j := a.registry.GetJob(jobName)

	if j == nil {
		log.V(1).Infof("EventJobScheduled(%s): Job not found in Registry")
		return
	}

	// Reassert there are no conflicts
	if a.hasConflict(j) {
		log.V(1).Infof("EventJobScheduled(%s): Local conflict found, cancelling Job", jobName)
		a.registry.CancelJob(jobName)
		return
	}

	log.Infof("EventJobScheduled(%s): Starting Job", j.Name)
	a.manager.StartJob(j)

	reversePeers := a.state.GetJobsByPeer(jobName)
	for _, peer := range reversePeers {
		log.V(1).Infof("EventJobScheduled(%s): Found unresolved offer for Peer(%s)", jobName, peer)

		if peerJob := a.registry.GetJob(peer); !a.hasConflict(peerJob) {
			log.Infof("EventJobScheduled(%s): Submitting JobBid for Peer(%s)", jobName, peer)
			jb := job.NewBid(peer, a.machine.BootId)
			a.registry.SubmitJobBid(jb)

			a.state.TrackBid(jb.JobName)
		} else {
			log.V(1).Infof("EventJobScheduled(%s): Would submit JobBid for Peer(%s), but local conflict exists", jobName, peer)
		}
	}
}

// Only call this from a locked AgentState context
func (a *Agent) bidForPossibleOffers() {
	for _, offer := range a.state.GetUnbadeOffers() {
		if !a.hasConflict(&offer.Job) && a.hasAllLocalPeers(&offer.Job) {
			log.Infof("Unscheduled Job(%s) has no local conflicts, submitting JobBid", offer.Job.Name)
			jb := job.NewBid(offer.Job.Name, a.machine.BootId)
			a.registry.SubmitJobBid(jb)

			a.state.TrackBid(jb.JobName)
		}
	}
}

func (a *Agent) HandleEventJobCancelled(ev event.Event) {
	//TODO(bcwaldon): We should check the context of the event before
	// making any changes to local systemd or the registry

	jobName := ev.Payload.(string)
	log.Infof("EventJobCancelled(%s): stopping Job", jobName)
	j := job.NewJob(jobName, nil, nil)
	a.manager.StopJob(j)

	a.state.Lock()
	defer a.state.Unlock()

	reversePeers := a.state.GetJobsByPeer(jobName)
	a.state.DropPeersJob(jobName)

	for _, peer := range reversePeers {
		log.Infof("EventJobCancelled(%s): cancelling Peer(%s) of Job", jobName, peer)
		a.registry.CancelJob(peer)
	}

	a.bidForPossibleOffers()
}

func parseDuration(d string) time.Duration {
	duration, err := time.ParseDuration(d)
	if err != nil {
		panic(err)
	}

	return duration
}

func intervalFromTTL(d time.Duration) time.Duration {
	return d / refreshInterval
}
