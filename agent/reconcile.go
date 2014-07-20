package agent

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
)

const (
	// time between triggering reconciliation routine
	reconcileInterval = 5 * time.Second

	taskTypeLoadJob       = "LoadJob"
	taskTypeUnloadJob     = "UnloadJob"
	taskTypeStartJob      = "StartJob"
	taskTypeStopJob       = "StopJob"
	taskTypeUnscheduleJob = "UnscheduleJob"
	taskTypeSubmitBid     = "SubmitBid"

	taskReasonScheduledButNotRunnable    = "job scheduled locally but unable to run"
	taskReasonScheduledButUnloaded       = "job scheduled here but not loaded"
	taskReasonLoadedButNotScheduled      = "job loaded but not scheduled here"
	taskReasonLoadedDesiredStateLaunched = "job currently loaded but desired state is launched"
	taskReasonLaunchedDesiredStateLoaded = "job currently launched but desired state is loaded"
	taskReasonPurgingAgent               = "purging agent"
	taskReasonAbleToResolveOffer         = "offer unresolved and able to run job"
)

type task struct {
	Type   string
	Job    *job.Job
	Reason string
}

func (t *task) String() string {
	var jName string
	if t.Job != nil {
		jName = t.Job.Name
	}
	return fmt.Sprintf("{Type:%s, Job:%s, Reason:%q}", t.Type, jName, t.Reason)
}

type offer struct {
	Bids pkg.Set
	Job  *job.Job
}

type offerCache map[string]*offer

func (oc *offerCache) add(name string, bids pkg.Set, j *job.Job) {
	(*oc)[name] = &offer{
		Bids: bids,
		Job:  j,
	}
}

func NewReconciler(reg registry.Registry, verifier *sign.SignatureVerifier) *AgentReconciler {
	return &AgentReconciler{reg, verifier, make(chan struct{})}
}

type AgentReconciler struct {
	reg      registry.Registry
	verifier *sign.SignatureVerifier

	rTrigger chan struct{}
}

// Run periodically attempts to reconcile the provided Agent until the stop
// channel is closed. Run will also reconcile in reaction to calls to Trigger.
// While a reconciliation is being attempted, calls to Trigger are ignored.
func (ar *AgentReconciler) Run(a *Agent, stop chan bool) {
	ticker := time.Tick(reconcileInterval)

	reconcile := func() {
		done := make(chan struct{})
		defer close(done)
		// While the reconciliation is running, flush the trigger channel in the background
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					select {
					case <-ar.rTrigger:
					case <-done:
						return
					}
				}
			}
		}()

		start := time.Now()
		ar.Reconcile(a)
		elapsed := time.Now().Sub(start)

		msg := fmt.Sprintf("AgentReconciler completed reconciliation in %s", elapsed)
		if elapsed > reconcileInterval {
			log.Warning(msg)
		} else {
			log.V(1).Info(msg)
		}
	}

	for {
		select {
		case <-stop:
			log.V(1).Info("AgentReconciler exiting due to stop signal")
			return
		case <-ticker:
			reconcile()
		case <-ar.rTrigger:
			reconcile()
		}
	}
}

// Trigger causes Reconcile to run if the Agent is running but is
// not currently reconciling.
func (ar *AgentReconciler) Trigger() {
	ar.rTrigger <- struct{}{}
}

// Reconcile drives the local Agent's state towards the desired state
// stored in the Registry. Reconcile also attempts to bid for any
// outstanding job offers that the local Agent can run.
func (ar *AgentReconciler) Reconcile(a *Agent) {
	ms := a.Machine.State()

	jobs, err := ar.reg.Jobs()
	if err != nil {
		log.Errorf("Failed fetching Jobs from Registry: %v", err)
		return
	}

	dAgentState, err := ar.desiredAgentState(jobs, ms.ID)
	if err != nil {
		log.Errorf("Unable to determine agent's desired state: %v", err)
		return
	}

	cAgentState, err := currentAgentState(a)
	if err != nil {
		log.Errorf("Unable to determine agent's current state: %v", err)
		return
	}

	for t := range ar.calculateTasksForJobs(&ms, dAgentState, cAgentState) {
		err := ar.doTask(a, t)
		if err != nil {
			log.Errorf("Failed resolving task, halting reconciliation: task=%s err=%q", t, err)
			return
		}
	}

	oCache, err := ar.currentOffers(jobs)
	if err != nil {
		log.Errorf("Unable to determine current state of offers: %v", err)
		return
	}

	for t := range ar.calculateTasksForOffers(oCache, dAgentState, &ms) {
		err := ar.doTask(a, t)
		if err != nil {
			log.Errorf("Failed resolving task, halting reconciliation: task=%s err=%q", t, err)
			return
		}
	}
}

// Purge attempts to unload all Jobs that have been loaded locally
func (ar *AgentReconciler) Purge(a *Agent) {
	cAgentState, err := currentAgentState(a)
	if err != nil {
		log.Errorf("Unable to determine agent's current state: %v", err)
		return
	}

	for _, cJob := range cAgentState.jobs {
		t := task{
			Type:   taskTypeUnloadJob,
			Job:    cJob,
			Reason: taskReasonPurgingAgent,
		}

		err := ar.doTask(a, &t)
		if err != nil {
			log.Errorf("Failed resolving task: task=%s err=%q", t, err)
		}
	}
}

// doTask takes action on an Agent based on the contents of a *task
func (ar *AgentReconciler) doTask(a *Agent, t *task) (err error) {
	switch t.Type {
	case taskTypeLoadJob:
		err = a.loadJob(t.Job)
	case taskTypeUnloadJob:
		a.unloadJob(t.Job.Name)
	case taskTypeStartJob:
		a.startJob(t.Job.Name)
	case taskTypeStopJob:
		a.stopJob(t.Job.Name)
	case taskTypeSubmitBid:
		ar.submitBid(t.Job.Name, a.Machine.State().ID)
	case taskTypeUnscheduleJob:
		err = ar.unscheduleJob(t.Job.Name, a.Machine.State().ID)
	default:
		err = fmt.Errorf("unrecognized task type %q", t.Type)
	}

	if err == nil {
		log.Infof("AgentReconciler completed task: task=%s", t)
	}

	return
}

func (ar *AgentReconciler) submitBid(jName, machID string) {
	ar.reg.SubmitJobBid(jName, machID)
}

func (ar *AgentReconciler) unscheduleJob(jName, machID string) error {
	return ar.reg.ClearJobTarget(jName, machID)
}

// desiredAgentState builds an *agentState object that represents what an
// Agent identified by the provided machine ID should currently be doing.
func (ar *AgentReconciler) desiredAgentState(jobs []job.Job, machID string) (*agentState, error) {
	as := agentState{jobs: make(map[string]*job.Job)}
	for _, j := range jobs {
		j := j
		if j.TargetState == job.JobStateInactive {
			continue
		}

		if j.TargetMachineID == "" || j.TargetMachineID != machID {
			continue
		}

		as.jobs[j.Name] = &j
	}

	return &as, nil
}

// currentAgentState builds an *agentState object that represents what an
// Agent is currently doing.
func currentAgentState(a *Agent) (*agentState, error) {
	jobs, err := a.jobs()
	if err != nil {
		return nil, err
	}

	as := agentState{jobs: jobs}
	return &as, nil
}

// calculateTasksForJobs compares the desired and current state of an Agent.
// The generateed tasks represent what should be done to make the desired
// state match the current state.
func (ar *AgentReconciler) calculateTasksForJobs(ms *machine.MachineState, dState, cState *agentState) <-chan *task {
	taskchan := make(chan *task)
	go func() {
		jobs := pkg.NewUnsafeSet()
		for cName := range cState.jobs {
			jobs.Add(cName)
		}

		for dName := range dState.jobs {
			jobs.Add(dName)
		}

		for _, name := range jobs.Values() {
			ar.calculateTasksForJob(ms, dState, cState, name, taskchan)
		}

		close(taskchan)
	}()

	return taskchan
}

func (ar *AgentReconciler) calculateTasksForJob(ms *machine.MachineState, dState, cState *agentState, jName string, taskchan chan *task) {
	var dJob, cJob *job.Job
	if dState != nil {
		dJob = dState.jobs[jName]
	}
	if cState != nil {
		cJob = cState.jobs[jName]
	}

	if dJob == nil && cJob == nil {
		log.Errorf("Desired state and current state of Job(%s) nil, not sure what to do", jName)
		return
	}

	if dJob == nil {
		taskchan <- &task{
			Type:   taskTypeUnloadJob,
			Job:    cJob,
			Reason: taskReasonLoadedButNotScheduled,
		}

		delete(cState.jobs, jName)
		return
	}

	if able, reason := ar.ableToRun(cState, ms, dJob); !able {
		log.Errorf("Unable to run locally-scheduled Job(%s): %s", jName, reason)

		taskchan <- &task{
			Type:   taskTypeUnscheduleJob,
			Job:    dJob,
			Reason: taskReasonScheduledButNotRunnable,
		}
		delete(dState.jobs, jName)

		taskchan <- &task{
			Type:   taskTypeUnloadJob,
			Job:    dJob,
			Reason: taskReasonScheduledButNotRunnable,
		}
		delete(cState.jobs, jName)

		return
	}

	if cJob == nil {
		taskchan <- &task{
			Type:   taskTypeLoadJob,
			Job:    dJob,
			Reason: taskReasonScheduledButUnloaded,
		}

		return
	}

	if cJob.State == nil {
		log.Errorf("Current state of Job(%s) unknown, unable to reconcile", jName)
		return
	}

	if dJob.State == nil {
		log.Errorf("Desired state of Job(%s) unknown, unable to reconcile", jName)
		return
	}

	if *cJob.State == dJob.TargetState {
		log.V(1).Infof("Desired state %q matches current state of Job(%s), nothing to do", *cJob.State, jName)
		return
	}

	if *cJob.State == job.JobStateLaunched && dJob.TargetState == job.JobStateLoaded {
		taskchan <- &task{
			Type:   taskTypeStopJob,
			Job:    cJob,
			Reason: taskReasonLaunchedDesiredStateLoaded,
		}
		return
	}

	if *cJob.State == job.JobStateLoaded && dJob.TargetState == job.JobStateLaunched {
		taskchan <- &task{
			Type:   taskTypeStartJob,
			Job:    cJob,
			Reason: taskReasonLoadedDesiredStateLaunched,
		}
		return
	}

	log.Errorf("Unable to determine how to reconcile Job(%s): desiredState=%#v currentState=%#V", jName, dJob, cJob)
}

func (ar *AgentReconciler) currentOffers(jobs []job.Job) (*offerCache, error) {
	jMap := make(map[string]*job.Job)
	for _, j := range jobs {
		j := j
		jMap[j.Name] = &j
	}

	uOffers, err := ar.reg.UnresolvedJobOffers()
	if err != nil {
		return nil, fmt.Errorf("failed fetching JobOffers from Registry: %v", err)
	}

	oCache := &offerCache{}
	for _, offer := range uOffers {
		bids, err := ar.reg.Bids(&offer)
		if err != nil {
			return nil, err
		}

		oCache.add(offer.Job.Name, bids, jMap[offer.Job.Name])
	}

	return oCache, nil
}

// calculateTasksForOffers compares the unresolved job offers and desired state
// of an Agent. The generated tasks represent which offers upon which the Agent
// should bid.
func (ar *AgentReconciler) calculateTasksForOffers(oCache *offerCache, dState *agentState, ms *machine.MachineState) <-chan *task {
	taskchan := make(chan *task)
	go func() {
		for oName, cache := range *oCache {
			if cache.Job == nil {
				log.Errorf("Unable to determine what to do about JobOffer(%s), Job is nil", oName)
				continue
			}
			ar.calculateTasksForOffer(dState, ms, cache.Job, cache.Bids, taskchan)
		}

		close(taskchan)
	}()

	return taskchan
}

func (ar *AgentReconciler) calculateTasksForOffer(dState *agentState, ms *machine.MachineState, j *job.Job, bids pkg.Set, taskchan chan *task) {
	if bids.Contains(ms.ID) {
		log.V(1).Infof("Bid already submitted for unresolved JobOffer(%s)", j.Name)
		return
	}

	if able, reason := ar.ableToRun(dState, ms, j); !able {
		log.V(1).Infof("Not bidding on Job(%s): %s", j.Name, reason)
		return
	}

	taskchan <- &task{
		Type:   taskTypeSubmitBid,
		Job:    j,
		Reason: taskReasonAbleToResolveOffer,
	}
}

// ableToRun determines if the Agent can run the provided Job based on
// the Agent's desired state. A boolean indicating whether this is the
// case or not is returned. The following criteria is used:
//   - Agent must meet the Job's machine target requirement (if any)
//   - Job must pass signature verification
//   - Agent must have all of the Job's required metadata (if any)
//   - Agent must have all required Peers of the Job scheduled locally (if any)
//   - Job must not conflict with any other Jobs scheduled to the agent
func (ar *AgentReconciler) ableToRun(as *agentState, ms *machine.MachineState, j *job.Job) (bool, string) {
	log.V(1).Infof("Attempting to determine if able to run Job(%s)", j.Name)

	if tgt, ok := j.RequiredTarget(); ok && !ms.MatchID(tgt) {
		return false, fmt.Sprintf("Agent ID %q does not match required %q", ms.ID, tgt)
	}

	if !ar.verifyJobSignature(j) {
		return false, "unable to verify signature"
	}

	log.V(1).Infof("Job(%s) has requirements: %s", j.Name, j.Requirements())

	metadata := j.RequiredTargetMetadata()
	if len(metadata) == 0 {
		log.V(1).Infof("Job(%s) has no required machine metadata", j.Name)
	} else {
		log.V(1).Infof("Job(%s) requires machine metadata: %v", j.Name, metadata)
		if !machine.HasMetadata(ms, metadata) {
			return false, "local Machine metadata insufficient"
		}
	}

	peers := j.Peers()
	if len(peers) == 0 {
		log.V(1).Infof("Job(%s) has no required peers", j.Name)
	} else {
		log.V(1).Infof("Job(%s) requires peers: %v", j.Name, peers)
		for _, peer := range peers {
			if !as.jobScheduled(peer) {
				return false, fmt.Sprintf("required peer Job(%s) is not scheduled locally", peer)
			}
		}
	}

	if cExists, cJobName := as.hasConflict(j.Name, j.Conflicts()); cExists {
		return false, fmt.Sprintf("found conflict with locally-scheduled Job(%s)", cJobName)
	}

	log.V(1).Infof("Determined local Agent is able to run Job(%s)", j.Name)
	return true, ""
}

// verifyJobSignature attempts to verify the integrity of the given Job by checking the
// signature against a SignatureSet stored in the Registry
func (ar *AgentReconciler) verifyJobSignature(j *job.Job) bool {
	if ar.verifier == nil {
		return true
	}
	ss, _ := ar.reg.JobSignatureSet(j.Name)
	ok, err := ar.verifier.VerifyJob(j, ss)
	if err != nil {
		log.V(1).Infof("Error verifying signature of Job(%s): %v", j.Name, err)
		return false
	} else if !ok {
		log.V(1).Infof("Job(%s) does not match signature", j.Name)
		return false
	}

	return true
}
