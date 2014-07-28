package engine

import (
	"fmt"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

const (
	taskTypeOfferJob      = "OfferJob"
	taskTypeResolveOffer  = "ResolveOffer"
	taskTypeUnscheduleJob = "UnscheduleJob"
	taskTypeScheduleJob   = "ScheduleJob"
)

type task struct {
	Type   string
	Reason string
	Job    *job.Job
}

func newTask(typ, reason string, j *job.Job) *task {
	return &task{Type: typ, Reason: reason, Job: j}
}

func (t *task) String() string {
	var jName string
	if t.Job != nil {
		jName = t.Job.Name
	}
	return fmt.Sprintf("{Type: %s, Job: %s, Reason: %q}", t.Type, jName, t.Reason)
}

type dumbReconciler struct{}

func (r *dumbReconciler) Reconcile(e *Engine) {
	log.V(1).Infof("Polling Registry for actionable work")

	clust, err := e.clusterState()
	if err != nil {
		log.Errorf("Failed getting current cluster state: %v", err)
		return
	}

	taskchan := make(chan *task)
	go calculateClusterTasks(taskchan, clust)

	for t := range taskchan {
		err = doTask(t, e)
		if err != nil {
			log.Errorf("Failed resolving task: task=%s err=%v", t, err)
		}
	}
}

func calculateClusterTasks(taskchan chan *task, clust *clusterState) {
	resolveJobOffer := func(jName, reason string) {
		clust.forgetOffer(jName)
		taskchan <- newTask(taskTypeResolveOffer, reason, &job.Job{Name: jName})
	}

	inactive := clust.inactiveJobs()
	needScheduling := clust.unscheduledLoadedJobs()
	loaded := clust.scheduledLoadedJobs()

	for _, j := range inactive {
		if j.Scheduled() {
			taskchan <- newTask(taskTypeUnscheduleJob, "target state inactive", j)
		}

		if clust.offerExists(j.Name) {
			resolveJobOffer(j.Name, "target state inactive")
		}
	}

	for _, j := range loaded {
		if clust.machineExists(j.TargetMachineID) {
			if clust.offerExists(j.Name) {
				resolveJobOffer(j.Name, "already scheduled")
			}
		} else {
			reason := fmt.Sprintf("target Machine(%s) went away", j.TargetMachineID)
			taskchan <- newTask(taskTypeUnscheduleJob, reason, j)

			needScheduling = append(needScheduling, j)
		}
	}

	for _, j := range needScheduling {
		reason := fmt.Sprintf("target state %s and Job not scheduled", j.TargetState)

		if !clust.offerExists(j.Name) {
			taskchan <- newTask(taskTypeOfferJob, reason, j)

			// wait for the next reconciliation attempt to resolve this offer
			continue
		}

		taskchan <- newTask(taskTypeScheduleJob, reason, j)
		clust.forgetOffer(j.Name)
	}

	// Deal with remaining JobOffers that do not have a corresponding Job
	for _, jName := range clust.unresolvedOffers() {
		resolveJobOffer(jName, "job does not exist")
	}

	close(taskchan)
}

func doTask(t *task, e *Engine) (err error) {
	switch t.Type {
	case taskTypeOfferJob:
		err = e.offerJob(t.Job)
	case taskTypeResolveOffer:
		err = e.resolveJobOffer(t.Job.Name)
	case taskTypeUnscheduleJob:
		err = e.unscheduleJob(t.Job.Name, t.Job.TargetMachineID)
	case taskTypeScheduleJob:
		if e.attemptScheduleJob(t.Job.Name) {
			err = e.resolveJobOffer(t.Job.Name)
		}
	default:
		err = fmt.Errorf("unrecognized task type %q", t.Type)
	}

	if err == nil {
		log.Infof("EngineReconciler completed task: %s", t)
	}

	return
}
