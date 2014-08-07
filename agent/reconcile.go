package agent

import (
	"fmt"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
)

const (
	// time between triggering reconciliation routine
	reconcileInterval = 5 * time.Second

	taskTypeLoadJob       = "LoadJob"
	taskTypeUnloadJob     = "UnloadJob"
	taskTypeStartJob      = "StartJob"
	taskTypeStopJob       = "StopJob"
	taskTypeUnscheduleJob = "UnscheduleJob"

	taskReasonScheduledButNotRunnable    = "job scheduled locally but unable to run"
	taskReasonScheduledButUnloaded       = "job scheduled here but not loaded"
	taskReasonLoadedButNotScheduled      = "job loaded but not scheduled here"
	taskReasonLoadedDesiredStateLaunched = "job currently loaded but desired state is launched"
	taskReasonLaunchedDesiredStateLoaded = "job currently launched but desired state is loaded"
	taskReasonPurgingAgent               = "purging agent"
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
	return fmt.Sprintf("{Type: %s, Job: %s, Reason: %q}", t.Type, jName, t.Reason)
}

func NewReconciler(reg registry.Registry, rStream registry.EventStream) (*AgentReconciler, error) {
	ar := AgentReconciler{reg, rStream}
	return &ar, nil
}

type AgentReconciler struct {
	reg     registry.Registry
	rStream registry.EventStream
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
	ms := a.Machine.State()

	jobs, err := ar.reg.Jobs()
	if err != nil {
		log.Errorf("Failed fetching Jobs from Registry: %v", err)
		return
	}

	dAgentState, err := ar.desiredAgentState(jobs, &ms)
	if err != nil {
		log.Errorf("Unable to determine agent's desired state: %v", err)
		return
	}

	cAgentState, err := ar.currentAgentState(a)
	if err != nil {
		log.Errorf("Unable to determine agent's current state: %v", err)
		return
	}

	for t := range ar.calculateTasksForJobs(dAgentState, cAgentState) {
		err := ar.doTask(a, t)
		if err != nil {
			log.Errorf("Failed resolving task, halting reconciliation: task=%s err=%q", t, err)
			return
		}
	}
}

// Purge attempts to unload all Jobs that have been loaded locally
func (ar *AgentReconciler) Purge(a *Agent) {
	cAgentState, err := ar.currentAgentState(a)
	if err != nil {
		log.Errorf("Unable to determine agent's current state: %v", err)
		return
	}

	for _, cJob := range cAgentState.Jobs {
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
	case taskTypeUnscheduleJob:
		err = ar.unscheduleJob(t.Job.Name, a.Machine.State().ID)
	default:
		err = fmt.Errorf("unrecognized task type %q", t.Type)
	}

	if err == nil {
		log.Infof("AgentReconciler completed task: %s", t)
	}

	return
}

func (ar *AgentReconciler) unscheduleJob(jName, machID string) error {
	return ar.reg.ClearJobTarget(jName, machID)
}

// desiredAgentState builds an *AgentState object that represents what an
// Agent identified by the provided machine ID should currently be doing.
func (ar *AgentReconciler) desiredAgentState(jobs []job.Job, ms *machine.MachineState) (*AgentState, error) {
	as := AgentState{
		MState: ms,
		Jobs:   make(map[string]*job.Job),
	}

	for _, j := range jobs {
		j := j

		if j.TargetMachineID == "" || j.TargetMachineID != ms.ID {
			continue
		}

		as.Jobs[j.Name] = &j
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

// calculateTasksForJobs compares the desired and current state of an Agent.
// The generateed tasks represent what should be done to make the desired
// state match the current state.
func (ar *AgentReconciler) calculateTasksForJobs(dState, cState *AgentState) <-chan *task {
	taskchan := make(chan *task)
	go func() {
		jobs := pkg.NewUnsafeSet()
		for cName := range cState.Jobs {
			jobs.Add(cName)
		}

		for dName := range dState.Jobs {
			jobs.Add(dName)
		}

		for _, name := range jobs.Values() {
			ar.calculateTasksForJob(dState, cState, name, taskchan)
		}

		close(taskchan)
	}()

	return taskchan
}

func (ar *AgentReconciler) calculateTasksForJob(dState, cState *AgentState, jName string, taskchan chan *task) {
	var dJob, cJob *job.Job
	if dState != nil {
		dJob = dState.Jobs[jName]
	}
	if cState != nil {
		cJob = cState.Jobs[jName]
	}

	if dJob == nil && cJob == nil {
		log.Errorf("Desired state and current state of Job(%s) nil, not sure what to do", jName)
		return
	}

	if dJob == nil || dJob.TargetState == job.JobStateInactive {
		taskchan <- &task{
			Type:   taskTypeUnloadJob,
			Job:    cJob,
			Reason: taskReasonLoadedButNotScheduled,
		}

		delete(cState.Jobs, jName)
		return
	}

	if able, reason := cState.AbleToRun(dJob); !able {
		log.Errorf("Unable to run locally-scheduled Job(%s): %s", jName, reason)

		taskchan <- &task{
			Type:   taskTypeUnscheduleJob,
			Job:    dJob,
			Reason: taskReasonScheduledButNotRunnable,
		}
		delete(dState.Jobs, jName)

		taskchan <- &task{
			Type:   taskTypeUnloadJob,
			Job:    dJob,
			Reason: taskReasonScheduledButNotRunnable,
		}
		delete(cState.Jobs, jName)

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

	if *cJob.State == job.JobStateInactive {
		taskchan <- &task{
			Type:   taskTypeLoadJob,
			Job:    dJob,
			Reason: taskReasonScheduledButUnloaded,
		}
	}

	if (*cJob.State == job.JobStateInactive || *cJob.State == job.JobStateLoaded) && dJob.TargetState == job.JobStateLaunched {
		taskchan <- &task{
			Type:   taskTypeStartJob,
			Job:    cJob,
			Reason: taskReasonLoadedDesiredStateLaunched,
		}
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

	log.Errorf("Unable to determine how to reconcile Job(%s): desiredState=%#v currentState=%#V", jName, dJob, cJob)
}
