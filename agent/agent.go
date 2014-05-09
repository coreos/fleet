package agent

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
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
	registry registry.Registry
	eStream  *registry.EventStream
	eBus     *event.EventBus
	machine  *machine.Machine
	ttl      time.Duration
	// verifier is used to verify the contents of a job's Unit.
	// A nil verifier implies that all Units are accepted.
	verifier *sign.SignatureVerifier

	state   *AgentState
	systemd *systemd.SystemdManager

	// channel used to shutdown any open connections/channels the Agent holds
	stop chan bool
}

func New(reg registry.Registry, eStream *registry.EventStream, mach *machine.Machine, ttl string, verifier *sign.SignatureVerifier) (*Agent, error) {
	ttldur, err := time.ParseDuration(ttl)
	if err != nil {
		return nil, err
	}

	mgr, err := systemd.NewSystemdManager(mach)
	if err != nil {
		return nil, err
	}

	state := NewState()
	eBus := event.NewEventBus()

	a := &Agent{reg, eStream, eBus, mach, ttldur, verifier, state, mgr, nil}

	hdlr := NewEventHandler(a)
	eBus.AddListener("agent", hdlr)

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
// 2. Discover any jobs that are scheduled locally, loading/starting them if they can run locally
// 3. Cache all unresolved job offers and bid for any that can be run locally
// The returned value is the etcd index at which the agent's presence was announced.
func (a *Agent) initialize() uint64 {
	log.Infof("Initializing Agent")

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

	machID := a.machine.State().ID
	loaded := map[string]job.Job{}
	launched := map[string]job.Job{}
	for _, j := range a.registry.GetAllJobs() {
		tm := a.registry.GetJobTarget(j.Name)
		if tm == "" || tm != machID {
			continue
		}

		if !a.AbleToRun(&j) {
			log.Infof("Unable to run Job(%s), unscheduling", j.Name)
			a.registry.ClearJobTarget(j.Name, machID)
			continue
		}

		ts := a.registry.GetJobTargetState(j.Name)
		if ts == nil || *ts == job.JobStateInactive {
			continue
		}

		loaded[j.Name] = j

		if *ts != job.JobStateLaunched {
			continue
		}

		launched[j.Name] = j
	}

	units, err := a.systemd.Units()
	if err != nil {
		log.Warningf("Failed determining what units are already loaded: %v", err)
	}

	for _, name := range units {
		if _, ok := loaded[name]; !ok {
			log.Infof("Unit(%s) should not be loaded here, unloading", name)
			a.systemd.StopJob(name)
			a.systemd.UnloadJob(name)
		}
	}

	for _, j := range loaded {
		a.state.TrackJob(&j)
		a.LoadJob(&j)

		if _, ok := launched[j.Name]; !ok {
			continue
		}

		a.StartJob(j.Name)
	}

	for _, jo := range a.registry.UnresolvedJobOffers() {
		// Everything we check against could change over time, so we track
		// all offers starting here for future bidding even if we are
		// currently unable to bid
		a.state.TrackOffer(jo)
		a.state.TrackJob(&jo.Job)
	}

	a.BidForPossibleJobs()

	return idx
}

// Stop all async processes the Agent is running
func (a *Agent) Stop() {
	log.Info("Stopping Agent")
	close(a.stop)
}

// Purge removes the Agent's state from the Registry
func (a *Agent) Purge() {
	// Continue heartbeating the agent's machine state while attempting to
	// stop all the locally-running jobs
	purged := make(chan bool)
	go a.Heartbeat(a.ttl, purged)

	machID := a.machine.State().ID

	for _, jobName := range a.state.ScheduledJobs() {
		log.Infof("Unscheduling Job(%s) from local machine", jobName)
		a.registry.ClearJobTarget(jobName, machID)
		log.Infof("Unloading Job(%s) from local machine", jobName)
		a.UnloadJob(jobName)
	}

	// Jobs have been stopped, the heartbeat can stop
	close(purged)

	log.Info("Removing Agent from Registry")
	if err := a.registry.RemoveMachineState(machID); err != nil {
		log.Errorf("Failed to remove Machine %s from Registry: %s", machID, err.Error())
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
		machID := a.Machine().State().ID
		launched := a.state.LaunchedJobs()
		for _, j := range launched {
			go a.registry.JobHeartbeat(j, machID, ttl)
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
	err := a.systemd.LoadJob(j)
	if err != nil {
		log.Errorf("Failed loading Job(%s) in systemd: %v", j.Name, err)
		return
	}

	// We must explicitly refresh the payload state, as the dbus
	// event listener does not send an event when we write a unit
	// file to disk.
	us, err := a.systemd.GetUnitState(j.Name)
	if err != nil {
		log.Errorf("Failed fetching state of Unit(%s): %v", j.Name, err)
		return
	}
	a.ReportUnitState(j.Name, us)
}

func (a *Agent) StartJob(jobName string) {
	a.state.SetTargetState(jobName, job.JobStateLaunched)

	machID := a.Machine().State().ID
	a.registry.JobHeartbeat(jobName, machID, a.ttl)

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
	a.ReportUnitState(jobName, nil)

	// Trigger rescheduling of all the peers of the job that was just unloaded
	machID := a.machine.State().ID
	for _, peer := range reversePeers {
		log.Infof("Unloading Peer(%s) of Job(%s)", peer, jobName)
		err := a.registry.ClearJobTarget(peer, machID)
		if err != nil {
			log.Errorf("Failed unloading Peer(%s) of Job(%s): %v", peer, jobName, err)
		}
	}
}

// Persist the state of the given Job into the Registry
func (a *Agent) ReportUnitState(jobName string, us *unit.UnitState) {
	if us == nil {
		err := a.registry.RemoveUnitState(jobName)
		if err != nil {
			log.Errorf("Failed to remove UnitState for job %s from Registry: %s", jobName, err.Error())
		}
	} else {
		a.registry.SaveUnitState(jobName, us)
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

	jb := job.NewBid(jobName, a.machine.State().ID)
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

// VerifyJob attempts to verify the integrity of the given Job by checking the
// signature against a SignatureSet stored in its repository.
func (a *Agent) VerifyJob(j *job.Job) bool {
	if a.verifier == nil {
		return true
	}
	ss := a.registry.GetSignatureSetOfJob(j.Name)
	ok, err := a.verifier.VerifyJob(j, ss)
	if err != nil {
		log.V(1).Infof("Error verifying signature of Job(%s): %v", j.Name, err)
		return false
	} else if !ok {
		log.V(1).Infof("Job(%s) does not match signature", j.Name)
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
	}

	log.Infof("Job(%s) has requirements: %s", j.Name, requirements)

	metadata := j.RequiredTargetMetadata()
	log.V(1).Infof("Job(%s) requires machine metadata: %v", j.Name, metadata)
	if !a.machine.HasMetadata(metadata) {
		log.Infof("Unable to run Job(%s), local Machine metadata insufficient", j.Name)
		return false
	}

	if tgt, ok := j.RequiredTarget(); ok && !a.machine.State().MatchID(tgt) {
		log.Infof("Agent does not meet machine target requirement for Job(%s)", j.Name)
		return false
	}

	peers := j.Peers()
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

	if conflicted, conflictedJobName := a.HasConflict(j.Name, j.Conflicts()); conflicted {
		log.Infof("Job(%s) has conflict with Job(%s)", j.Name, conflictedJobName)
		return false
	}

	return true
}

// Determine if all necessary peers of a Job are scheduled to this Agent
func (a *Agent) peerScheduledHere(jobName, peerName string) bool {
	log.V(1).Infof("Looking for target of Peer(%s)", peerName)

	//FIXME: ideally the machine would use its own knowledge rather than calling GetJobTarget
	if tgt := a.registry.GetJobTarget(peerName); tgt == "" || tgt != a.machine.State().ID {
		log.V(1).Infof("Peer(%s) of Job(%s) not scheduled here", peerName, jobName)
		return false
	}

	log.V(1).Infof("Peer(%s) of Job(%s) scheduled here", peerName, jobName)
	return true
}

// HasConflict determines whether there are any known conflicts with the given argument
func (a *Agent) HasConflict(potentialJobName string, potentialConflicts []string) (bool, string) {
	// Iterate through each Job that is scheduled here, asserting two things
	for existingJobName, existingConflicts := range a.state.Conflicts {
		if !a.state.ScheduledHere(existingJobName) {
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
