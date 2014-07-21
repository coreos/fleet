package agent

import (
	"encoding/json"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/unit"
)

const (
	// TTL to use with all state pushed to Registry
	DefaultTTL = "30s"
)

// The Agent owns all of the coordination between the Registry, the local
// Machine, and the local UnitManager.
type Agent struct {
	registry registry.Registry
	um       unit.UnitManager
	uGen     *unit.UnitStateGenerator
	Machine  machine.Machine
	ttl      time.Duration
	// verifier is used to verify the contents of a job's Unit.
	// A nil verifier implies that all Units are accepted.
	verifier *sign.SignatureVerifier

	state *AgentState
}

func New(mgr unit.UnitManager, uGen *unit.UnitStateGenerator, reg registry.Registry, mach machine.Machine, ttl string, verifier *sign.SignatureVerifier) (*Agent, error) {
	ttldur, err := time.ParseDuration(ttl)
	if err != nil {
		return nil, err
	}

	a := &Agent{reg, mgr, uGen, mach, ttldur, verifier, NewState()}
	return a, nil
}

func (a *Agent) MarshalJSON() ([]byte, error) {
	data := struct {
		State *AgentState
	}{
		State: a.state,
	}
	return json.Marshal(data)
}

// Heartbeat updates the Registry periodically with an acknowledgement of the
// Jobs this Agent is expected to be running.
func (a *Agent) Heartbeat(stop chan bool) {
	a.heartbeatJobs(a.ttl, stop)
}

// Initialize prepares the Agent for normal operation by doing three things:
// 1. Announce presence to the Registry
// 2. Discover any jobs that are scheduled locally, loading/starting them if they can run locally
// 3. Cache all unresolved job offers and bid for any that can be run locally
func (a *Agent) Initialize() {
	log.Infof("Initializing Agent")

	// Lock the state early so we can decide what the Agent needs to do
	// without the risk of conflicting with any of its other moving parts
	a.state.Lock()
	defer a.state.Unlock()

	machID := a.Machine.State().ID
	loaded := map[string]job.Job{}
	launched := map[string]job.Job{}
	jobs, _ := a.registry.Jobs()
	for _, j := range jobs {
		if j.TargetMachineID == "" || j.TargetMachineID != machID {
			continue
		}

		if !a.ableToRun(&j) {
			log.Infof("Unable to run Job(%s), unscheduling", j.Name)
			a.registry.ClearJobTarget(j.Name, machID)
			continue
		}

		if j.TargetState == job.JobStateInactive {
			continue
		}

		loaded[j.Name] = j

		if j.TargetState != job.JobStateLaunched {
			continue
		}

		launched[j.Name] = j
	}

	units, err := a.um.Units()
	if err != nil {
		log.Warningf("Failed determining what units are already loaded: %v", err)
	}

	for _, name := range units {
		if _, ok := loaded[name]; !ok {
			log.Infof("Unit(%s) should not be loaded here, unloading", name)
			go func() {
				a.um.Stop(name)
				a.um.Unload(name)
			}()
		}
	}

	for _, j := range loaded {
		a.state.TrackJob(&j)
		a.loadJob(&j)

		if _, ok := launched[j.Name]; !ok {
			continue
		}

		a.startJobUnlocked(j.Name)
	}

	offers, _ := a.registry.UnresolvedJobOffers()
	for _, jo := range offers {
		// Everything we check against could change over time, so we track
		// all offers starting here for future bidding even if we are
		// currently unable to bid
		a.state.TrackOffer(jo)
		a.state.TrackJob(&jo.Job)
	}

	a.bidForPossibleJobs()
}

// Purge removes the Agent's state from the Registry
func (a *Agent) Purge() {
	a.state.Lock()
	scheduled := a.state.ScheduledJobs()
	a.state.Unlock()

	machID := a.Machine.State().ID
	for _, jobName := range scheduled {
		log.Infof("Unloading Job(%s) from local machine", jobName)
		a.unloadJob(jobName)

		//TODO(bcwaldon): RemoveUnitState is unsafe and could destroy state
		// published by another agent
		log.Infof("Destroying Job(%s)'s state in Registry", jobName)
		a.registry.RemoveUnitState(jobName)

		log.Infof("Unscheduling Job(%s) from local machine", jobName)
		a.registry.ClearJobTarget(jobName, machID)
	}
}

func (a *Agent) heartbeatJobs(ttl time.Duration, stop chan bool) {
	heartbeat := func() {
		machID := a.Machine.State().ID
		launched := a.state.LaunchedJobs()
		for _, j := range launched {
			go a.registry.JobHeartbeat(j, machID, ttl)
		}
	}

	interval := ttl / 2
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

// loadJob hands the given Job to systemd without acquiring the
// state mutex. The caller is responsible for acquiring it.
func (a *Agent) loadJob(j *job.Job) {
	log.Infof("Loading Job(%s)", j.Name)
	a.state.SetTargetState(j.Name, job.JobStateLoaded)
	a.uGen.Subscribe(j.Name)
	err := a.um.Load(j.Name, j.Unit)
	if err != nil {
		log.Errorf("Failed loading Job(%s): %v", j.Name, err)
		return
	}
}

// StartJob starts the indicated Job after first acquiring the state mutex
func (a *Agent) StartJob(jobName string) {
	a.state.Lock()
	defer a.state.Unlock()

	a.startJobUnlocked(jobName)
}

// startJobUnlocked starts the indicated Job without acquiring the state
// mutex. The caller is responsible for acquiring it.
func (a *Agent) startJobUnlocked(jobName string) {
	a.state.SetTargetState(jobName, job.JobStateLaunched)

	machID := a.Machine.State().ID
	a.registry.JobHeartbeat(jobName, machID, a.ttl)

	go func() {
		a.um.Start(jobName)
	}()
}

// StopJob stops the indicated Job after first acquiring the state mutex
func (a *Agent) StopJob(jobName string) {
	a.state.Lock()
	defer a.state.Unlock()
	a.stopJobUnlocked(jobName)
}

// stopJobUnlocked stops the indicated Job without acquiring the state
// mutex. The caller is responsible for acquiring it.
func (a *Agent) stopJobUnlocked(jobName string) {
	a.state.SetTargetState(jobName, job.JobStateLoaded)
	a.registry.ClearJobHeartbeat(jobName)

	go func() {
		a.um.Stop(jobName)
	}()
}

// unloadJob stops and expunges the indicated Job without acquiring the
// state mutex. The caller is responsible for acquiring it.
func (a *Agent) unloadJob(jobName string) {
	go func() {
		a.um.Stop(jobName)
		a.uGen.Unsubscribe(jobName)
		a.um.Unload(jobName)
	}()

	a.registry.ClearJobHeartbeat(jobName)
	a.registry.RemoveUnitState(jobName)

	// Grab the peers of the Job before we destroy the state
	reversePeers := a.state.GetJobsByPeer(jobName)

	a.state.PurgeJob(jobName)

	// Trigger rescheduling of all the peers of the job that was just unloaded
	machID := a.Machine.State().ID
	for _, peer := range reversePeers {
		log.Infof("Unloading Peer(%s) of Job(%s)", peer, jobName)
		err := a.registry.ClearJobTarget(peer, machID)
		if err != nil {
			log.Errorf("Failed unloading Peer(%s) of Job(%s): %v", peer, jobName, err)
		}
	}
}

// ReportUnitState attaches the current state of the Agent's Machine to the given
// unit.UnitState object, then persists that state in the Registry
func (a *Agent) ReportUnitState(jobName string, us *unit.UnitState) {
	if us == nil {
		log.Infof("Job(%s): purging UnitState from Registry", jobName)
		err := a.registry.RemoveUnitState(jobName)
		if err != nil {
			log.Errorf("Failed to remove UnitState for job %s from Registry: %s", jobName, err.Error())
		}
	} else {
		us.MachineID = a.Machine.State().ID
		log.Infof("Job(%s): pushing UnitState (loadState=%s, activeState=%s, subState=%s) to Registry", jobName, us.LoadState, us.ActiveState, us.SubState)
		a.registry.SaveUnitState(jobName, us)
	}
}

// MaybeBid bids for the given JobOffer only if the Agent determines that it is able
// to run the JobOffer's Job
func (a *Agent) MaybeBid(jo job.JobOffer) {
	a.state.Lock()
	defer a.state.Unlock()

	// Everything we check against could change over time, so we track all
	// offers starting here for future bidding even if we can't bid now
	a.state.TrackOffer(jo)
	a.state.TrackJob(&jo.Job)

	if !a.ableToRun(&jo.Job) {
		log.Infof("EventJobOffered(%s): not all criteria met, not bidding", jo.Job.Name)
		return
	}

	log.Infof("EventJobOffered(%s): passed all criteria, submitting JobBid", jo.Job.Name)
	a.bid(jo.Job.Name)
}

// bidForPossibleJobs submits bids for all unresolved offers whose Jobs
// can be run locally
func (a *Agent) bidForPossibleJobs() {
	offers := a.state.GetOffersWithoutBids()

	log.V(1).Infof("Checking %d unbade offers", len(offers))
	for i := range offers {
		offer := offers[i]
		log.V(1).Infof("Checking ability to run Job(%s)", offer.Job.Name)
		if a.ableToRun(&offer.Job) {
			log.V(1).Infof("Able to run Job(%s), submitting bid", offer.Job.Name)
			a.bid(offer.Job.Name)
		} else {
			log.V(1).Infof("Still unable to run Job(%s)", offer.Job.Name)
		}
	}
}

// Submit a bid for the given Job
func (a *Agent) bid(jobName string) {
	log.Infof("Submitting JobBid for Job(%s)", jobName)
	a.registry.SubmitJobBid(jobName, a.Machine.State().ID)
	a.state.TrackBid(jobName)
}

// verifyJobSignature attempts to verify the integrity of the given Job by checking the
// signature against a SignatureSet stored in its repository.
func (a *Agent) verifyJobSignature(j *job.Job) bool {
	if a.verifier == nil {
		return true
	}
	ss, _ := a.registry.JobSignatureSet(j.Name)
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

// bidForPossiblePeers submits bids for all known peers of the provided job that can
// be run locally
func (a *Agent) bidForPossiblePeers(jobName string) {
	peers := a.state.GetJobsByPeer(jobName)

	for _, peer := range peers {
		log.V(1).Infof("Found unresolved offer for Peer(%s) of Job(%s)", peer, jobName)

		peerJob, err := a.registry.Job(peer)
		if err != nil {
			log.Errorf("Failed fetching Job(%s) from Registry: %v", peer, err)
			return
		}

		if peerJob == nil {
			log.V(1).Infof("Unable to find Peer(%s) of Job(%s) in Registry", peer, jobName)
			return
		}

		if !a.ableToRun(peerJob) {
			log.V(1).Infof("Unable to run Peer(%s) of Job(%s), not bidding", peer, jobName)
			return
		}

		a.bid(peer)
	}
}

// ableToRun determines if the Agent can run the provided Job, and returns a boolean indicating
// whether this is the case. There are five criteria for an Agent to be eligible to run a Job:
//   - Job must pass signature verification
//   - agent must have all of the Job's required metadata (if any)
//   - agent must meet the Job's machine target requirement (if any)
//   - agent must have all required Peers of the Job scheduled locally (if any)
//   - Job must not conflict with any other Jobs scheduled to the agent
func (a *Agent) ableToRun(j *job.Job) bool {
	if !a.verifyJobSignature(j) {
		log.V(1).Infof("Failed to verify Job(%s)", j.Name)
		return false
	}

	log.Infof("Job(%s) has requirements: %s", j.Name, j.Requirements())

	metadata := j.RequiredTargetMetadata()
	if len(metadata) == 0 {
		log.V(1).Infof("Job(%s) has no required machine metadata", j.Name)
	} else {
		log.V(1).Infof("Job(%s) requires machine metadata: %v", j.Name, metadata)
		ms := a.Machine.State()
		if !machine.HasMetadata(&ms, metadata) {
			log.Infof("Unable to run Job(%s): local Machine metadata insufficient", j.Name)
			return false
		}
	}

	if tgt, ok := j.RequiredTarget(); ok && !a.Machine.State().MatchID(tgt) {
		log.Infof("Unable to run Job(%s): agent does not meet machine target requirement (%s)", j.Name, tgt)
		return false
	}

	peers := j.Peers()
	if len(peers) == 0 {
		log.V(1).Infof("Job(%s) has no required peers", j.Name)
	} else {
		log.V(1).Infof("Job(%s) requires peers: %v", j.Name, peers)
		for _, peer := range peers {
			if !a.peerScheduledHere(j.Name, peer) {
				log.Infof("Unable to run Job(%s): required Peer(%s) is not scheduled locally", j.Name, peer)
				return false
			}
		}
	}

	if conflicted, conflictedJobName := a.HasConflict(j.Name, j.Conflicts()); conflicted {
		log.Infof("Unable to run Job(%s): conflict with Job(%s)", j.Name, conflictedJobName)
		return false
	}

	return true
}

// Determine if all necessary peers of a Job are scheduled to this Agent
func (a *Agent) peerScheduledHere(jobName, peerName string) bool {
	log.V(1).Infof("Looking for target of Peer(%s)", peerName)

	j, err := a.registry.Job(peerName)
	if err != nil {
		log.Errorf("Failed retrieving Job(%s) from Registry: %v", peerName, err)
		return false
	} else if j == nil {
		return false
	}

	if j.TargetMachineID == "" || j.TargetMachineID != a.Machine.State().ID {
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

// JobScheduledElsewhere clears all state related to the indicated
// job before bidding for all oustanding jobs that can be run locally.
func (a *Agent) JobScheduledElsewhere(jobName string) {
	a.state.Lock()
	defer a.state.Unlock()

	log.Infof("Dropping offer and bid for Job(%s) from cache", jobName)
	a.state.PurgeOffer(jobName)

	log.Infof("Purging Job(%s) data from cache", jobName)
	a.state.PurgeJob(jobName)

	log.Infof("Checking outstanding job offers")
	a.bidForPossibleJobs()
}

// JobScheduledLocally clears all state related to the indicated
// job's offers/bids before attempting to load and possibly start
// the job. The ability to run the job will be revalidated before
// loading, and unscheduled if such validation fails.
func (a *Agent) JobScheduledLocally(jobName string) {
	a.state.Lock()
	defer a.state.Unlock()

	log.Infof("Dropping offer and bid for Job(%s) from cache", jobName)
	a.state.PurgeOffer(jobName)

	j, err := a.registry.Job(jobName)
	if err != nil {
		log.Errorf("Failed fetching Job(%s) from Registry: %v", jobName, err)
		return
	}

	if j == nil {
		log.Errorf("Unable to find Job(%s) in Registry", jobName)
		return
	}

	if !a.ableToRun(j) {
		log.Infof("Unable to run locally-scheduled Job(%s), unscheduling", jobName)
		a.registry.ClearJobTarget(jobName, a.Machine.State().ID)
		a.state.PurgeJob(jobName)
		return
	}

	a.loadJob(j)

	log.Infof("Bidding for all possible peers of Job(%s)", j.Name)
	a.bidForPossiblePeers(j.Name)

	if j.TargetState != job.JobStateLaunched {
		return
	}

	log.Infof("Job(%s) loaded, now starting it", j.Name)
	a.startJobUnlocked(j.Name)
}

// JobUnscheduled attempts to unload the indicated job only
// if it were scheduled here in the first place, otherwise
// the event is ignored. If unloading is necessary, all jobs
// that can be run locally will also be bid upon.
func (a *Agent) JobUnscheduled(jobName string) {
	a.state.Lock()
	defer a.state.Unlock()

	if !a.state.ScheduledHere(jobName) {
		log.V(1).Infof("Job(%s) not scheduled here, ignoring", jobName)
		return
	}

	log.Infof("Unloading Job(%s)", jobName)
	a.unloadJob(jobName)

	log.Infof("Checking outstanding JobOffers")
	a.bidForPossibleJobs()
}
