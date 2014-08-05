package engine

import (
	"fmt"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

const (
	taskTypeOfferJob           = "OfferJob"
	taskTypeResolveOffer       = "ResolveOffer"
	taskTypeUnscheduleJob      = "UnscheduleJob"
	taskTypeAttemptScheduleJob = "AttemptScheduleJob"
)

type task struct {
	Type   string
	Reason string
	Job    *job.Job
}

func (t *task) String() string {
	var jName string
	if t.Job != nil {
		jName = t.Job.Name
	}
	return fmt.Sprintf("{Type: %s, Job: %s, Reason: %q}", t.Type, jName, t.Reason)
}

type Reconciler struct{}

func (r *Reconciler) Reconcile(e *Engine, stop chan struct{}) {
	log.V(1).Infof("Polling Registry for actionable work")

	clust, err := e.clusterState()
	if err != nil {
		log.Errorf("Failed getting current cluster state: %v", err)
		return
	}

	for t := range calculateClusterTasks(clust, stop) {
		err = doTask(t, e)
		if err != nil {
			log.Errorf("Failed resolving task: task=%s err=%v", t, err)
		}
	}
}

func calculateClusterTasks(clust *clusterState, stopchan chan struct{}) (taskchan chan *task) {
	taskchan = make(chan *task)

	send := func(typ, reason string, j *job.Job) bool {
		select {
		case <-stopchan:
			return false
		default:
		}

		taskchan <- &task{Type: typ, Reason: reason, Job: j}
		return true
	}

	resolveJobOffer := func(jName, reason string) bool {
		clust.forgetOffer(jName)
		return send(taskTypeResolveOffer, reason, &job.Job{Name: jName})
	}

	go func() {
		defer close(taskchan)

		inactive := clust.inactiveJobs()
		needScheduling := clust.unscheduledLoadedJobs()
		loaded := clust.scheduledLoadedJobs()

		for _, j := range inactive {
			if j.Scheduled() {
				if !send(taskTypeUnscheduleJob, "target state inactive", j) {
					return
				}
			}

			if clust.offerExists(j.Name) {
				if !resolveJobOffer(j.Name, "target state inactive") {
					return
				}
			}
		}

		for _, j := range loaded {
			if clust.machineExists(j.TargetMachineID) {
				if clust.offerExists(j.Name) {
					if !resolveJobOffer(j.Name, "already scheduled") {
						return
					}
				}
			} else {
				reason := fmt.Sprintf("target Machine(%s) went away", j.TargetMachineID)
				if !send(taskTypeUnscheduleJob, reason, j) {
					return
				}

				needScheduling = append(needScheduling, j)
			}
		}

		for _, j := range needScheduling {
			reason := fmt.Sprintf("target state %s and Job not scheduled", j.TargetState)

			if !clust.offerExists(j.Name) {
				if !send(taskTypeOfferJob, reason, j) {
					return
				}

				// wait for the next reconciliation attempt to resolve this offer
				continue
			}

			if !send(taskTypeAttemptScheduleJob, reason, j) {
				return
			}
			clust.forgetOffer(j.Name)
		}

		// Deal with remaining JobOffers that do not have a corresponding Job
		for _, jName := range clust.unresolvedOffers() {
			if !resolveJobOffer(jName, "job does not exist") {
				return
			}
		}
	}()

	return
}

func doTask(t *task, e *Engine) (err error) {
	switch t.Type {
	case taskTypeOfferJob:
		err = e.offerJob(t.Job)
	case taskTypeResolveOffer:
		err = e.resolveJobOffer(t.Job.Name)
	case taskTypeUnscheduleJob:
		err = e.unscheduleJob(t.Job.Name, t.Job.TargetMachineID)
	case taskTypeAttemptScheduleJob:
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
