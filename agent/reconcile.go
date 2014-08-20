package agent

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
)

const (
	// time between triggering reconciliation routine
	reconcileInterval = 5 * time.Second
)

func NewReconciler(reg registry.Registry, rStream registry.EventStream) (*AgentReconciler, error) {
	taskManager := newTaskManager()
	ar := AgentReconciler{reg, rStream, taskManager}
	return &ar, nil
}

type AgentReconciler struct {
	reg      registry.Registry
	rStream  registry.EventStream
	tManager *taskManager
}

// Run periodically attempts to reconcile the provided Agent until the stop
// channel is closed. Run will also reconcile in reaction to calls to Trigger.
// While a reconciliation is being attempted, calls to Trigger are ignored.
func (ar *AgentReconciler) Run(a *Agent, stop chan bool) {
	ticker := time.Tick(reconcileInterval)

	reconcile := func() {
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

	trigger := make(chan struct{})
	go func() {
		abort := make(chan struct{})
		select {
		case <-stop:
			close(abort)
		case <-ar.rStream.Next(abort):
			trigger <- struct{}{}
		}
	}()

	for {
		select {
		case <-stop:
			log.V(1).Info("AgentReconciler exiting due to stop signal")
			return
		case <-ticker:
			reconcile()
		case <-trigger:
			reconcile()
		}
	}
}

// Reconcile drives the local Agent's state towards the desired state
// stored in the Registry.
func (ar *AgentReconciler) Reconcile(a *Agent) {
	dAgentState, err := ar.desiredAgentState(a, ar.reg)
	if err != nil {
		log.Errorf("Unable to determine agent's desired state: %v", err)
		return
	}

	cAgentState, err := ar.currentAgentState(a)
	if err != nil {
		log.Errorf("Unable to determine agent's current state: %v", err)
		return
	}

	for tc := range ar.calculateTaskChainsForJobs(dAgentState, cAgentState) {
		ar.launchTaskChain(tc, a)
	}
}

// Purge attempts to unload all Jobs that have been loaded locally
func (ar *AgentReconciler) Purge(a *Agent) {
	for {
		cAgentState, err := ar.currentAgentState(a)
		if err != nil {
			log.Errorf("Unable to determine agent's current state: %v", err)
			return
		}

		if len(cAgentState.Jobs) == 0 {
			return
		}

		for _, cJob := range cAgentState.Jobs {
			cJob := cJob
			t := task{
				typ:    taskTypeUnloadJob,
				reason: taskReasonPurgingAgent,
			}

			tc := newTaskChain(cJob, t)
			ar.launchTaskChain(tc, a)
		}

		time.Sleep(time.Second)
	}
}

// desiredAgentState builds an *AgentState object that represents what the
// provided Agent should currently be doing.
func (ar *AgentReconciler) desiredAgentState(a *Agent, reg registry.Registry) (*AgentState, error) {
	units, err := reg.Units()
	if err != nil {
		log.Errorf("Failed fetching Units from Registry: %v", err)
		return nil, err
	}

	sUnits, err := reg.Schedule()
	if err != nil {
		log.Errorf("Failed fetching schedule from Registry: %v", err)
		return nil, err
	}

	ms := a.Machine.State()
	as := AgentState{
		MState: &ms,
		Jobs:   make(map[string]*job.Job),
	}

	sUnitMap := make(map[string]*job.ScheduledUnit)
	for _, sUnit := range sUnits {
		sUnit := sUnit
		sUnitMap[sUnit.Name] = &sUnit
	}

	for _, u := range units {
		sUnit, ok := sUnitMap[u.Name]
		if !ok || sUnit.TargetMachineID == "" || sUnit.TargetMachineID != ms.ID {
			continue
		}

		as.Jobs[u.Name] = &job.Job{
			Name:            u.Name,
			Unit:            u.Unit,
			TargetState:     u.TargetState,
			TargetMachineID: sUnit.TargetMachineID,
		}
	}

	return &as, nil
}

// currentAgentState builds an *AgentState object that represents what an
// Agent is currently doing.
func (ar *AgentReconciler) currentAgentState(a *Agent) (*AgentState, error) {
	jobs, err := a.jobs()
	if err != nil {
		return nil, err
	}

	ms := a.Machine.State()
	as := AgentState{
		MState: &ms,
		Jobs:   jobs,
	}

	return &as, nil
}

// calculateTaskChainsForJobs compares the desired and current state of an Agent.
// The generated taskChains represent what should be done to make the desired
// state match the current state.
func (ar *AgentReconciler) calculateTaskChainsForJobs(dState, cState *AgentState) <-chan taskChain {
	tcChan := make(chan taskChain)
	go func() {
		jobs := pkg.NewUnsafeSet()
		for cName := range cState.Jobs {
			jobs.Add(cName)
		}

		for dName := range dState.Jobs {
			jobs.Add(dName)
		}

		for _, name := range jobs.Values() {
			tc := ar.calculateTaskChainForJob(dState, cState, name)
			if tc == nil {
				continue
			}
			tcChan <- *tc
		}

		close(tcChan)
	}()

	return tcChan
}

func (ar *AgentReconciler) calculateTaskChainForJob(dState, cState *AgentState, jName string) *taskChain {
	var dJob, cJob *job.Job
	if dState != nil {
		dJob = dState.Jobs[jName]
	}
	if cState != nil {
		cJob = cState.Jobs[jName]
	}

	if dJob == nil && cJob == nil {
		log.Errorf("Desired state and current state of Job(%s) nil, not sure what to do", jName)
		return nil
	}

	if dJob == nil || dJob.TargetState == job.JobStateInactive {
		delete(cState.Jobs, jName)

		t := task{
			typ:    taskTypeUnloadJob,
			reason: taskReasonLoadedButNotScheduled,
		}

		tc := newTaskChain(cJob, t)
		return &tc
	}

	if cJob == nil {
		t := task{
			typ:    taskTypeLoadJob,
			reason: taskReasonScheduledButUnloaded,
		}
		tc := newTaskChain(dJob, t)

		// as an optimization, queue the job for launching immediately after loading
		if dJob.TargetState == job.JobStateLaunched {
			tc.Add(task{
				typ:    taskTypeStartJob,
				reason: taskReasonLoadedDesiredStateLaunched,
			})
		}

		return &tc
	}

	if cJob.State == nil {
		log.Errorf("Current state of Job(%s) unknown, unable to reconcile", jName)
		return nil
	}

	if *cJob.State == dJob.TargetState {
		log.V(1).Infof("Desired state %q matches current state of Job(%s), nothing to do", *cJob.State, jName)
		return nil
	}

	tc := newTaskChain(dJob)
	if *cJob.State == job.JobStateInactive {
		tc.Add(task{
			typ:    taskTypeLoadJob,
			reason: taskReasonScheduledButUnloaded,
		})
	}

	if (*cJob.State == job.JobStateInactive || *cJob.State == job.JobStateLoaded) && dJob.TargetState == job.JobStateLaunched {
		tc.Add(task{
			typ:    taskTypeStartJob,
			reason: taskReasonLoadedDesiredStateLaunched,
		})
	}

	if *cJob.State == job.JobStateLaunched && dJob.TargetState == job.JobStateLoaded {
		tc.Add(task{
			typ:    taskTypeStopJob,
			reason: taskReasonLaunchedDesiredStateLoaded,
		})
	}

	if len(tc.tasks) == 0 {
		log.Errorf("Unable to determine how to reconcile Job(%s): desiredState=%#v currentState=%#v", jName, dJob, cJob)
		return nil
	}

	return &tc
}

func (ar *AgentReconciler) launchTaskChain(tc taskChain, a *Agent) {
	log.V(1).Infof("AgentReconciler attempting task chain: %s", tc)
	reschan, err := ar.tManager.Do(tc, a)
	if err != nil {
		log.Infof("AgentReconciler task chain failed: chain=%s err=%v", tc, err)
		return
	}

	go func() {
		for res := range reschan {
			if res.err == nil {
				log.Infof("AgentReconciler completed task: type=%s job=%s reason=%q", res.task.typ, tc.job.Name, res.task.reason)
			} else {
				log.Infof("AgentReconciler task failed: type=%s job=%s reason=%q err=%v", res.task.typ, tc.job.Name, res.task.reason, res.err)
			}
		}
	}()
}
