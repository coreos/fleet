package engine

import (
	"fmt"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"
)

const (
	taskTypeUnscheduleJob      = "UnscheduleJob"
	taskTypeAttemptScheduleJob = "AttemptScheduleJob"
)

type task struct {
	Type      string
	Reason    string
	JobName   string
	MachineID string
}

func (t *task) String() string {
	return fmt.Sprintf("{Type: %s, JobName: %s, MachineID: %s, Reason: %q}", t.Type, t.JobName, t.MachineID, t.Reason)
}

func NewReconciler() *Reconciler {
	return &Reconciler{
		sched: &dumbScheduler{},
	}
}

type Reconciler struct {
	sched Scheduler
}

func (r *Reconciler) Reconcile(e *Engine, stop chan struct{}) {
	log.V(1).Infof("Polling Registry for actionable work")

	clust, err := e.clusterState()
	if err != nil {
		log.Errorf("Failed getting current cluster state: %v", err)
		return
	}

	for t := range r.calculateClusterTasks(clust, stop) {
		err = doTask(t, e)
		if err != nil {
			log.Errorf("Failed resolving task: task=%s err=%v", t, err)
		}
	}
}

func (r *Reconciler) calculateClusterTasks(clust *clusterState, stopchan chan struct{}) (taskchan chan *task) {
	taskchan = make(chan *task)

	send := func(typ, reason, jName, machID string) bool {
		select {
		case <-stopchan:
			return false
		default:
		}

		taskchan <- &task{Type: typ, Reason: reason, JobName: jName, MachineID: machID}
		return true
	}

	go func() {
		defer close(taskchan)

		inactive := clust.inactiveJobs()
		needScheduling := clust.unscheduledLoadedJobs()
		loaded := clust.scheduledLoadedJobs()

		for _, j := range inactive {
			if j.Scheduled() {
				if !send(taskTypeUnscheduleJob, "target state inactive", j.Name, j.TargetMachineID) {
					return
				}
			}
		}

		for _, j := range loaded {
			if !clust.machineExists(j.TargetMachineID) {
				reason := fmt.Sprintf("target Machine(%s) went away", j.TargetMachineID)
				if !send(taskTypeUnscheduleJob, reason, j.Name, j.TargetMachineID) {
					return
				}

				needScheduling = append(needScheduling, j)
			}
		}

		for _, j := range needScheduling {
			reason := fmt.Sprintf("target state %s and Job not scheduled", j.TargetState)

			dec, err := r.sched.Decide(clust, j)
			if err != nil {
				log.Infof("Unable to schedule Job(%s): %v", err)
				continue
			}

			j.TargetMachineID = dec.machineID

			if !send(taskTypeAttemptScheduleJob, reason, j.Name, dec.machineID) {
				return
			}
		}
	}()

	return
}

func doTask(t *task, e *Engine) (err error) {
	switch t.Type {
	case taskTypeUnscheduleJob:
		err = e.unscheduleJob(t.JobName, t.MachineID)
	case taskTypeAttemptScheduleJob:
		e.attemptScheduleJob(t.JobName, t.MachineID)
	default:
		err = fmt.Errorf("unrecognized task type %q", t.Type)
	}

	if err == nil {
		log.Infof("EngineReconciler completed task: %s", t)
	}

	return
}
