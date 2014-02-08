package agent

import (
	"fmt"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/systemd"
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
	registry      *registry.Registry
	events        *event.EventBus
	machine       *machine.Machine
	ttl           time.Duration
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
		log.V(1).Infof("Clearing JobState(%s) from Registry", j.Name)
		a.registry.SaveJobState(j.Name, nil)

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
	// Explicitly heartbeat immediately to push state to the
	// Registry as quickly as possible
	a.registry.SetMachineState(a.machine, a.ttl)

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
	a.state.TrackJobConflicts(j.Name, j.Payload.Conflicts())

	log.Infof("Starting Job(%s)", j.Name)
	a.systemd.StartJob(j)
}

// Inform the Registry that a Job must be rescheduled
func (a *Agent) RescheduleJob(j *job.Job) {
	log.V(2).Infof("Stopping Job(%s)", j.Name)
	a.registry.UnscheduleJob(j.Name)

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
	a.state.DropJobConflicts(jobName)
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

	peers := jo.Job.Payload.Peers()
	log.V(2).Infof("Tracking peers of JobOffer(%s): %v", jo.Job.Name, peers)
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
	requirements := j.Requirements()
	if len(requirements) == 0 {
		log.V(1).Infof("Job(%s) has no requirements", j.Name)
		return true
	}

	var reqString string
	for key, slice := range requirements {
		reqString += fmt.Sprintf("%s = [", key)
		for _, val := range slice {
			reqString += fmt.Sprintf("%s, ", val)
		}
		reqString += fmt.Sprint("] ")
	}

	log.V(1).Infof("Job(%s) has requirements: %s", j.Name, reqString)

	metadata := extractMachineMetadata(requirements)
	log.V(1).Infof("Job(%s) requires machine metadata: %v", j.Name, metadata)
	if !a.machine.HasMetadata(metadata) {
		log.V(1).Infof("Unable to run Job(%s), local Machine metadata insufficient", j.Name)
		return false
	}

	bootID, ok := requirements["ConditionMachineBootID"]
	if ok && len(bootID) > 0 && a.machine.BootId == bootID[0] {
		log.V(1).Infof("Agent does not pass MachineBootID condition for Job(%s)", j.Name)
		return false
	}

	peers := j.Payload.Peers()
	if len(peers) > 0 {
		log.V(1).Infof("Asserting required Peers %v of Job(%s) are scheduled locally", peers, j.Name)
		for _, peer := range peers {
			if !a.peerScheduledHere(j.Name, peer) {
				log.V(1).Infof("Required Peer(%s) of Job(%s) is not scheduled locally", peer, j.Name)
				return false
			}
		}
	} else {
		log.V(2).Infof("Job(%s) has no peers to worry about", j.Name)
	}

	if conflicted, conflictedJobName := a.state.HasConflict(j.Name, j.Payload.Conflicts()); conflicted {
		log.V(1).Infof("Job(%s) has conflict with Job(%s)", j.Name, conflictedJobName)
		return false
	}

	return true
}

// Return all machine-related metadata from a job requirements map
func extractMachineMetadata(requirements map[string][]string) map[string][]string {
	metadata := make(map[string][]string)

	for key, values := range requirements {
		if !strings.HasPrefix(key, "MachineMetadata") {
			continue
		}

		// Strip off leading 'MachineMetadata'
		key = key[15:]

		if len(values) == 0 {
			log.V(2).Infof("Machine metadata requirement %s provided no values, ignoring.", key)
			continue
		}

		metadata[key] = values
	}

	return metadata
}

// Determine if all necessary peers of a Job are scheduled to this Agent
func (a *Agent) peerScheduledHere(jobName, peerName string) bool {
	log.V(1).Infof("Looking for target of Peer(%s)", peerName)

	//FIXME: ideally the machine would use its own knowledge rather than calling GetJobTarget
	if tgt := a.registry.GetJobTarget(peerName); tgt == nil || tgt.BootId != a.machine.BootId {
		log.V(1).Infof("Peer(%s) of Job(%s) not scheduled here", peerName, jobName)
		return false
	}

	log.V(1).Infof("Peer(%s) of Job(%s) scheduled here", peerName, jobName)
	return true
}
