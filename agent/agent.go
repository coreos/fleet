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
	"github.com/coreos/coreinit/systemd"
)

const (
	// TTL to use with all state pushed to Registry
	DefaultTTL = "30s"

	// Refresh TTLs at 1/2 the TTL length
	refreshInterval = 2
)

// The Agent owns all of the coordination between the Registry, the local
// Machine, and the local SystemdManager.
type Agent struct {
	registry   *registry.Registry
	events     *event.EventBus
	machine    *machine.Machine
	ttl        time.Duration
	systemdPrefix string

	state   *AgentState
	systemd *systemd.SystemdManager

	// channel used to shutdown any open connections/channels the Agent holds
	stop chan bool
}

func New(registry *registry.Registry, events *event.EventBus, machine *machine.Machine, ttl, unitPrefix string) (*Agent, error) {
	ttldur, err := time.ParseDuration(ttl)
	if err != nil {
		return nil, err
	}

	state := NewState()
	mgr := systemd.NewSystemdManager(machine, unitPrefix)

	return &Agent{registry, events, machine, ttldur, unitPrefix, state, mgr, nil}, nil
}

// Access Agent's machine field
func (a *Agent) Machine() *machine.Machine {
	return a.machine
}

// Trigger all async processes the Agent intends to run
func (a *Agent) Run() {
	a.stop = make(chan bool)

	handler := NewEventHandler(a)
	a.events.AddListener("agent", a.machine, handler)

	go a.systemd.Publish(a.events, a.stop)
	go a.Heartbeat(a.ttl, a.stop)

	// Block until we receive a stop signal
	<-a.stop

	a.events.RemoveListener("agent", a.machine)
}

// Stop all async processes the Agent is running
func (a *Agent) Stop() {
	log.V(1).Info("Stopping Agent")
	close(a.stop)
}

// Clear any presence data from the Registry
func (a *Agent) Purge() {
	log.V(1).Info("Removing Agent from Registry")
	err := a.registry.RemoveMachineState(a.machine)
	if err != nil {
		log.Errorf("Failed to remove Machine %s from Registry: %s", a.machine.BootId, err.Error())
	}

	for _, j := range a.registry.GetAllJobsByMachine(a.machine) {
		offer := job.NewOfferFromJob(j)
		log.V(2).Infof("Publishing JobOffer(%s)", offer.Job.Name)
		a.registry.CreateJobOffer(offer)
		log.Infof("Published JobOffer(%s)", offer.Job.Name)
	}
}

// Periodically report to the Registry at an interval equal to
// half of the provided ttl. Stop reporting when the provided
// channel is closed.
func (a *Agent) Heartbeat(ttl time.Duration, stop chan bool) {
	interval := ttl / refreshInterval
	for true {
		select {
		case <-stop:
			log.V(2).Info("MachineHeartbeat exiting due to stop signal")
			return
		case <-time.Tick(interval):
			log.V(2).Info("MachineHeartbeat tick")
			a.registry.SetMachineState(a.machine, a.ttl)
		}
	}
}

// Instruct the Agent to start the provided Job
func (a *Agent) StartJob(j *job.Job) {
	log.Infof("Starting Job(%s)", j.Name)
	a.systemd.StartJob(j)
}

// Inform the Registry that a Job must be rescheduled
func (a *Agent) RescheduleJob(j *job.Job) {
	log.V(2).Infof("Stopping Job(%s)", j.Name)
	a.registry.StopJob(j.Name)

	offer := job.NewOfferFromJob(*j)
	log.V(2).Infof("Publishing JobOffer(%s)", offer.Job.Name)
	a.registry.CreateJobOffer(offer)
	log.Infof("Published JobOffer(%s)", offer.Job.Name)
}

// Instruct the Agent to stop the provided Job and
// all of its peers
func (a *Agent) StopJob(jobName string) {
	log.Infof("Stopping Job(%s)", jobName)
	a.systemd.StopJob(jobName)
	a.ReportJobState(jobName, nil)

	a.state.Lock()
	reversePeers := a.state.GetJobsByPeer(jobName)
	a.state.DropPeersJob(jobName)
	a.state.Unlock()

	for _, peer := range reversePeers {
		log.Infof("Stopping Peer(%s) of Job(%s)", peer, jobName)
		a.registry.StopJob(peer)
	}
}

// Persist the state of the given Job into the Registry
func (a *Agent) ReportJobState(jobName string, jobState *job.JobState) {
	if jobState == nil {
		err := a.registry.RemoveJobState(jobName)
		if err != nil {
			log.V(1).Infof("Failed to remove JobState from Registry: %s", jobName, err.Error())
		}
	} else {
		a.registry.SaveJobState(jobName, jobState)
	}
}

// Submit all possible bids for unresolved offers
func (a *Agent) BidForPossibleJobs() {
	a.state.Lock()
	offers := a.state.GetUnbadeOffers()
	a.state.Unlock()

	log.V(2).Infof("Checking %d unbade offers", len(offers))
	for i, _ := range offers {
		offer := offers[i]
		log.V(2).Infof("Checking ability to run Job(%s)", offer.Job.Name)
		if a.AbleToRun(&offer.Job) {
			log.V(2).Infof("Able to run Job(%s), submitting bid", offer.Job.Name)
			a.Bid(offer.Job.Name)
		} else {
			log.V(2).Infof("Still unable to run Job(%s)", offer.Job.Name)
		}
	}
}

// Submit a bid for the given Job
func (a *Agent) Bid(jobName string) {
	log.Infof("Submitting JobBid for Job(%s)", jobName)

	jb := job.NewBid(jobName, a.machine.BootId)
	a.registry.SubmitJobBid(jb)

	a.state.Lock()
	defer a.state.Unlock()

	a.state.TrackBid(jb.JobName)
}

// Instruct the Agent that an offer has been created and must
// be tracked until it is resolved
func (a *Agent) TrackOffer(jo job.JobOffer) {
	a.state.Lock()
	defer a.state.Unlock()

	log.V(2).Infof("Tracking JobOffer(%s)", jo.Job.Name)
	a.state.TrackOffer(jo)

	a.state.TrackJobPeers(jo.Job.Name, jo.Job.Payload.Peers())
}

// Instruct the Agent that the given offer has been resolved
// and may be ignored in future conflict calculations
func (a *Agent) OfferResolved(jobName string) {
	a.state.Lock()
	defer a.state.Unlock()

	log.V(2).Infof("Dropping JobOffer(%s)", jobName)
	a.state.DropOffer(jobName)

	a.state.DropBid(jobName)
}

// Pull a Job and its payload from the Registry
func (a *Agent) FetchJob(jobName string) *job.Job {
	log.V(1).Infof("Fetching Job(%s) from Registry", jobName)
	j := a.registry.GetJob(jobName)
	if j == nil {
		log.V(1).Infof("Job not found in Registry")
	}
	return j
}

// Submit all possible bids for known peers of the provided job
func (a *Agent) BidForPossiblePeers(jobName string) {
	a.state.Lock()
	peers := a.state.GetJobsByPeer(jobName)
	a.state.Unlock()

	for _, peer := range peers {
		log.V(1).Infof("Found unresolved offer for Peer(%s) of Job(%s)", peer, jobName)

		peerJob := a.FetchJob(peer)
		if peerJob != nil && a.AbleToRun(peerJob) {
			log.Infof("Submitting bid for Peer(%s) of Job(%s)", peer, jobName)
			a.Bid(peer)
		} else {
			log.V(1).Infof("Unable to bid for Peer(%s) of Job(%s)", peer, jobName)
		}
	}
}

// Determine if the Agent can run the provided Job
func (a *Agent) AbleToRun(j *job.Job) bool {
	metadata := extractMachineMetadata(j.Requirements())
	if !a.machine.HasMetadata(metadata) {
		log.V(1).Infof("Unable to run Job(%s), local Machine metadata insufficient", j.Name)
		return false
	}

	if a.hasConflict(j) {
		log.V(1).Infof("Unable to run Job(%s), local Job conflict", j.Name)
		return false
	}

	if !a.hasAllLocalPeers(j) {
		log.V(1).Infof("Unable to run Job(%s), necessary peer Jobs are not running locally", j.Name)
		return false
	}

	return true
}

// Determine if all necessary peers of a Job are scheduled to this Agent
func (a *Agent) hasAllLocalPeers(j *job.Job) bool {
	for _, peerName := range j.Payload.Peers() {
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

// Determine whether a given Job conflicts with any other relevant Jobs
func (a *Agent) hasConflict(j *job.Job) bool {
	requirements := j.Requirements()

	var reqString string
	for key, slice := range requirements {
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
		singleton, ok := requirements["MachineSingleton"]
		return ok && singleton[0] == "true"
	}

	hasProvides := func(j *job.Job) bool {
		provides, ok := requirements["Provides"]
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

		for _, provide := range requirements["Provides"] {
			for _, otherProvide := range requirements["Provides"] {
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

		for _, provide := range requirements["Provides"] {
			for _, offerProvide := range requirements["Provides"] {
				if provide == offerProvide {
					log.V(1).Infof("Outstanding JobBid(%s) already provides '%s'", offer.Job.Name, provide)
					return true
				}
			}
		}
	}

	return false
}

// Return all machine-related metadata from a job requirements map
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
