package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/systemd"
)

const (
	// TTL to use with all state pushed to Registry
	DefaultTTL = "30s"

	// Refresh TTLs at 1/2 the TTL length
	refreshInterval = 2

	// Machine metadata key for the deprecated `require` flag
	requireFlagMachineMetadata = "MachineMetadata"

	// Machine metadata key in the unit file, without the X- prefix
	fleetXConditionMachineMetadata = "ConditionMachineMetadata"
)

// The Agent owns all of the coordination between the Registry, the local
// Machine, and the local SystemdManager.
type Agent struct {
	registry *registry.Registry
	eStream  *registry.EventStream
	eBus     *event.EventBus
	machine  *machine.Machine
	ttl      time.Duration
	// verifier is used to verify job payload. A nil one implies that
	// all payloads are accepted.
	verifier *sign.SignatureVerifier

	state   *AgentState
	systemd *systemd.SystemdManager

	// channel used to shutdown any open connections/channels the Agent holds
	stop chan bool
}

func New(reg *registry.Registry, eStream *registry.EventStream, mach *machine.Machine, ttl string, verifier *sign.SignatureVerifier) (*Agent, error) {
	ttldur, err := time.ParseDuration(ttl)
	if err != nil {
		return nil, err
	}

	state := NewState()
	mgr := systemd.NewSystemdManager(mach)

	eBus := event.NewEventBus()

	a := &Agent{reg, eStream, eBus, mach, ttldur, verifier, state, mgr, nil}

	hdlr := NewEventHandler(a)
	bootID := mach.State().BootID
	eBus.AddListener("agent", bootID, hdlr)

	return a, nil
}

// Access Agent's machine field
func (a *Agent) Machine() *machine.Machine {
	return a.machine
}

func (a *Agent) MarshalJSON() ([]byte, error) {
	data := struct {
		Systemd *systemd.SystemdManager
		State   *AgentState
	}{
		Systemd: a.systemd,
		State:   a.state,
	}
	return json.Marshal(data)
}

// Trigger all async processes the Agent intends to run
func (a *Agent) Run() {
	a.stop = make(chan bool)

	idx := a.initialize()
	go a.eBus.Listen(a.stop)
	go a.eStream.Stream(idx, a.eBus.Channel, a.stop)

	go a.systemd.Publish(a.eBus, a.stop)

	go a.Heartbeat(a.ttl, a.stop)
	go a.HeartbeatJobs(a.ttl, a.stop)
}

// initialize prepares the Agent for normal operation by doing three things:
// 1. Announce presence to the Registry, tracking the etcd index of the operation
// 2. Discover any jobs that are scheduled locally and load/start them
// 3. Cache all unresolved job offers and bid for any that can be run locally
// The returned value is the etcd index at which the agent's presence was announced.
func (a *Agent) initialize() uint64 {
	log.Infof("Initializing Agent")
	a.machine.RefreshState()

	var idx uint64
	wait := time.Second
	for {
		var err error
		if idx, err = a.registry.SetMachineState(a.machine.State(), a.ttl); err == nil {
			log.V(1).Infof("Heartbeat succeeded")
			break
		}
		log.V(1).Infof("Failed heartbeat, retrying in %v", wait)
		time.Sleep(wait)
	}

	for _, j := range a.registry.GetAllJobs() {
		tm := a.registry.GetJobTarget(j.Name)
		if tm == "" || tm != a.machine.State().BootID {
			continue
		}

		ts := a.registry.GetJobTargetState(j.Name)
		if ts != nil && *ts != job.JobStateLoaded && *ts != job.JobStateLaunched {
			continue
		}

		a.state.TrackJob(&j)
		a.LoadJob(&j)

		if *ts != job.JobStateLaunched {
			continue
		}

		a.StartJob(j.Name)
	}

	for _, jo := range a.UnresolvedJobOffers() {
		// Everything we check against could change over time, so we track
		// all offers starting here for future bidding even if we are
		// currently unable to bid
		a.state.TrackOffer(jo)
	}

	a.BidForPossibleJobs()

	return idx
}

// Stop all async processes the Agent is running
func (a *Agent) Stop() {
	log.Info("Stopping Agent")
	close(a.stop)

	// Continue heartbeating the agent's machine state while attempting to
	// stop all the locally-running jobs
	purged := make(chan bool)
	close(purged)

	go a.Heartbeat(a.ttl, purged)

	for _, jobName := range a.state.ScheduledJobs() {
		log.Infof("Unloading Job(%s) from local machine", jobName)
		a.UnloadJob(jobName)
	}
}

// Periodically report to the Registry at an interval equal to
// half of the provided ttl. Stop reporting when the provided
// channel is closed. Failed attempts to report state to the
// Registry are retried twice before moving on to the next
// reporting interval.
func (a *Agent) Heartbeat(ttl time.Duration, stop chan bool) {
	attempt := func(attempts int, f func() error) (err error) {
		if attempts < 1 {
			return fmt.Errorf("attempts argument must be 1 or greater, got %d", attempts)
		}

		// The amount of time the retry mechanism waits after a failed attempt
		// doubles following each failure. This is a simple exponential backoff.
		sleep := time.Second

		for i := 1; i <= attempts; i++ {
			err = f()
			if err == nil || i == attempts {
				break
			}

			sleep = sleep * 2
			log.V(1).Infof("function returned err, retrying in %v: %v", sleep, err)
			time.Sleep(sleep)
		}

		return err
	}

	heartbeat := func() error {
		_, err := a.registry.SetMachineState(a.machine.State(), ttl)
		return err
	}

	interval := ttl / refreshInterval
	ticker := time.Tick(interval)
	for {
		select {
		case <-stop:
			log.V(1).Info("MachineHeartbeat exiting due to stop signal")
			return
		case <-ticker:
			log.V(1).Info("MachineHeartbeat tick")
			a.machine.RefreshState()
			if err := attempt(3, heartbeat); err != nil {
				log.Errorf("Failed heartbeat after 3 attempts: %v", err)
			}
		}
	}
}

func (a *Agent) HeartbeatJobs(ttl time.Duration, stop chan bool) {
	heartbeat := func() {
		bootID := a.Machine().State().BootID
		launched := a.state.LaunchedJobs()
		for _, j := range launched {
			go a.registry.JobHeartbeat(j, bootID, ttl)
		}
	}

	interval := ttl / refreshInterval
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

func (a *Agent) LoadJob(j *job.Job) {
	log.Infof("Loading Job(%s)", j.Name)
	a.state.SetTargetState(j.Name, job.JobStateLoaded)
	a.systemd.LoadJob(j)

	// We must explicitly refresh the payload state, as the dbus
	// event listener does not send an event when we write a unit
	// file to disk.
	ps, err := a.systemd.GetPayloadState(j.Name)
	if err != nil {
		log.Errorf("Failed fetching state of Payload(%s)", j.Name)
		return
	}
	a.ReportPayloadState(j.Name, ps)
}

func (a *Agent) StartJob(jobName string) {
	a.state.SetTargetState(jobName, job.JobStateLaunched)

	bootID := a.Machine().State().BootID
	a.registry.JobHeartbeat(jobName, bootID, a.ttl)

	a.systemd.StartJob(jobName)
}

func (a *Agent) StopJob(jobName string) {
	a.state.SetTargetState(jobName, job.JobStateLoaded)
	a.registry.ClearJobHeartbeat(jobName)
	a.systemd.StopJob(jobName)
}

func (a *Agent) UnloadJob(jobName string) {
	a.StopJob(jobName)

	reversePeers := a.state.GetJobsByPeer(jobName)

	a.state.PurgeJob(jobName)
	a.systemd.UnloadJob(jobName)

	// The dbus event systemd will not trigger an event telling
	// us that the unit has been unloaded, so we must explicitly
	// clear what is in the Registry.
	a.ReportPayloadState(jobName, nil)

	// Trigger rescheduling of all the peers of the job that was just unloaded
	bootID := a.machine.State().BootID
	for _, peer := range reversePeers {
		log.Infof("Unloading Peer(%s) of Job(%s)", peer, jobName)
		err := a.registry.ClearJobTarget(peer, bootID)
		if err != nil {
			log.Errorf("Failed unloading Peer(%s) of Job(%s): %v", peer, jobName, err)
		}
	}
}

// Persist the state of the given Job into the Registry
func (a *Agent) ReportPayloadState(jobName string, ps *job.PayloadState) {
	if ps == nil {
		err := a.registry.RemovePayloadState(jobName)
		if err != nil {
			log.Errorf("Failed to remove PayloadState from Registry: %s", jobName, err.Error())
		}
	} else {
		a.registry.SavePayloadState(jobName, ps)
	}
}

// Submit all possible bids for unresolved offers
func (a *Agent) BidForPossibleJobs() {
	offers := a.state.GetOffersWithoutBids()

	log.V(1).Infof("Checking %d unbade offers", len(offers))
	for i, _ := range offers {
		offer := offers[i]
		log.V(1).Infof("Checking ability to run Job(%s)", offer.Job.Name)
		if a.AbleToRun(&offer.Job) {
			log.V(1).Infof("Able to run Job(%s), submitting bid", offer.Job.Name)
			a.Bid(offer.Job.Name)
		} else {
			log.V(1).Infof("Still unable to run Job(%s)", offer.Job.Name)
		}
	}
}

// Submit a bid for the given Job
func (a *Agent) Bid(jobName string) {
	log.Infof("Submitting JobBid for Job(%s)", jobName)

	jb := job.NewBid(jobName, a.machine.State().BootID)
	a.registry.SubmitJobBid(jb)

	a.state.TrackBid(jb.JobName)
}

// Pull a Job and its payload from the Registry
func (a *Agent) FetchJob(jobName string) *job.Job {
	log.V(1).Infof("Fetching Job(%s) from Registry", jobName)
	j := a.registry.GetJob(jobName)
	if j == nil {
		log.V(1).Infof("Job not found in Registry")
		return nil
	}
	return j
}

// Verify a Job through SignatureSet
func (a *Agent) VerifyJob(j *job.Job) bool {
	if a.verifier == nil {
		return true
	}

	payload := j.Payload
	s := a.registry.GetSignatureSetOfPayload(payload.Name)
	ok, err := a.verifier.VerifyPayload(&payload, s)
	if !ok || err != nil {
		log.V(1).Infof("Payload(%s) doesn't fit its signature: %v", payload.Name, err)
		return false
	}
	return true
}

// Submit all possible bids for known peers of the provided job
func (a *Agent) BidForPossiblePeers(jobName string) {
	peers := a.state.GetJobsByPeer(jobName)

	for _, peer := range peers {
		log.V(1).Infof("Found unresolved offer for Peer(%s) of Job(%s)", peer, jobName)

		peerJob := a.FetchJob(peer)
		if peerJob != nil && a.AbleToRun(peerJob) {
			a.Bid(peer)
		} else {
			log.V(1).Infof("Unable to bid for Peer(%s) of Job(%s)", peer, jobName)
		}
	}
}

// Determine if the Agent can run the provided Job
func (a *Agent) AbleToRun(j *job.Job) bool {
	if !a.VerifyJob(j) {
		log.V(1).Infof("Failed to verify Job(%s)", j.Name)
		return false
	}

	requirements := j.Requirements()
	if len(requirements) == 0 {
		log.V(1).Infof("Job(%s) has no requirements", j.Name)
		return true
	}

	log.Infof("Job(%s) has requirements: %s", j.Name, requirements)

	metadata := extractMachineMetadata(requirements)
	log.V(1).Infof("Job(%s) requires machine metadata: %v", j.Name, metadata)
	if !a.machine.HasMetadata(metadata) {
		log.Infof("Unable to run Job(%s), local Machine metadata insufficient", j.Name)
		return false
	}

	bootID, ok := requirements[job.FleetXConditionMachineBootID]
	if ok && len(bootID) > 0 && !a.machine.State().MatchBootID(bootID[0]) {
		log.Infof("Agent does not pass MachineBootID condition for Job(%s)", j.Name)
		return false
	}

	peers := j.Payload.Peers()
	if len(peers) > 0 {
		log.V(1).Infof("Asserting required Peers %v of Job(%s) are scheduled locally", peers, j.Name)
		for _, peer := range peers {
			if !a.peerScheduledHere(j.Name, peer) {
				log.Infof("Required Peer(%s) of Job(%s) is not scheduled locally", peer, j.Name)
				return false
			}
		}
	} else {
		log.V(1).Infof("Job(%s) has no peers to worry about", j.Name)
	}

	if conflicted, conflictedJobName := a.HasConflict(j.Name, j.Payload.Conflicts()); conflicted {
		log.Infof("Job(%s) has conflict with Job(%s)", j.Name, conflictedJobName)
		return false
	}

	return true
}

// Return all machine-related metadata from a job requirements map
func extractMachineMetadata(requirements map[string][]string) map[string][]string {
	metadata := make(map[string][]string)
	for key, values := range requirements {
		// Deprecated syntax added to the metadata via the old `--require` flag.
		if strings.HasPrefix(key, requireFlagMachineMetadata) {
			if len(values) == 0 {
				log.V(2).Infof("Machine metadata requirement %s provided no values, ignoring.", key)
				continue
			}

			metadata[key[15:]] = values
		} else if key == fleetXConditionMachineMetadata {
			for _, valuePair := range values {
				s := strings.Split(valuePair, "=")

				if len(s) != 2 {
					log.V(2).Infof("Machine metadata requirement %q has invalid format, ignoring.", valuePair)
					continue
				}

				if len(s[0]) == 0 || len(s[1]) == 0 {
					log.V(2).Infof("Machine metadata requirement %q provided no values, ignoring.", valuePair)
					continue
				}

				var mValues []string
				if mv, ok := metadata[s[0]]; ok {
					mValues = mv
				}

				metadata[s[0]] = append(mValues, s[1])
			}
		}
	}

	return metadata
}

// Determine if all necessary peers of a Job are scheduled to this Agent
func (a *Agent) peerScheduledHere(jobName, peerName string) bool {
	log.V(1).Infof("Looking for target of Peer(%s)", peerName)

	//FIXME: ideally the machine would use its own knowledge rather than calling GetJobTarget
	if tgt := a.registry.GetJobTarget(peerName); tgt == "" || tgt != a.machine.State().BootID {
		log.V(1).Infof("Peer(%s) of Job(%s) not scheduled here", peerName, jobName)
		return false
	}

	log.V(1).Infof("Peer(%s) of Job(%s) scheduled here", peerName, jobName)
	return true
}

func (a *Agent) UnresolvedJobOffers() []job.JobOffer {
	return a.registry.UnresolvedJobOffers()
}

// HasConflict determines whether there are any known conflicts with the given argument
func (a *Agent) HasConflict(potentialJobName string, potentialConflicts []string) (bool, string) {
	// Iterate through each Job that is scheduled here or has already been bid upon, asserting two things
	for existingJobName, existingConflicts := range a.state.Conflicts {
		if !a.state.HasBid(existingJobName) && !a.state.ScheduledHere(existingJobName) {
			continue
		}

		// 1. Each tracked Job does not conflict with the potential conflicts
		for _, pc := range potentialConflicts {
			if globMatches(pc, existingJobName) {
				return true, existingJobName
			}
		}

		// 2. The new Job does not conflict with any of the tracked conflicts
		for _, ec := range existingConflicts {
			if globMatches(ec, potentialJobName) {
				return true, existingJobName
			}
		}
	}

	return false, ""
}
