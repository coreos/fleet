package engine

import (
	"fmt"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

const (
	taskTypeUnscheduleJob       = "UnscheduleJob"
	taskTypeAttemptScheduleUnit = "AttemptScheduleUnit"
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
		sched: &leastLoadedScheduler{},
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

		agents := clust.agents()

		for _, j := range clust.jobs {
			if !j.Scheduled() {
				continue
			}

			decide := func() (unschedule bool, reason string) {
				if j.TargetState == job.JobStateInactive {
					unschedule = true
					reason = "target state inactive"
					return
				}

				as, ok := agents[j.TargetMachineID]
				if !ok {
					unschedule = true
					reason = fmt.Sprintf("target Machine(%s) went away", j.TargetMachineID)
					return
				}

				var able bool
				if able, reason = as.AbleToRun(j); !able {
					unschedule = true
					reason = fmt.Sprintf("target Machine(%s) unable to run job")
					return
				}

				return
			}

			unschedule, reason := decide()
			if !unschedule {
				continue
			}

			if !send(taskTypeUnscheduleJob, reason, j.Name, j.TargetMachineID) {
				return
			}

			clust.unschedule(j.Name)
		}

		for _, j := range clust.jobs {
			if j.Scheduled() || j.TargetState == job.JobStateInactive {
				continue
			}

			dec, err := r.sched.Decide(clust, j)
			if err != nil {
				log.V(1).Infof("Unable to schedule Job(%s): %v", j.Name, err)
				continue
			}

			reason := fmt.Sprintf("target state %s and unit not scheduled", j.TargetState)
			if !send(taskTypeAttemptScheduleUnit, reason, j.Name, dec.machineID) {
				return
			}

			clust.schedule(j.Name, dec.machineID)
		}
	}()

	return
}

func doTask(t *task, e *Engine) (err error) {
	switch t.Type {
	case taskTypeUnscheduleJob:
		err = e.unscheduleJob(t.JobName, t.MachineID)
	case taskTypeAttemptScheduleUnit:
		e.attemptScheduleUnit(t.JobName, t.MachineID)
	default:
		err = fmt.Errorf("unrecognized task type %q", t.Type)
	}

	if err == nil {
		log.Infof("EngineReconciler completed task: %s", t)
	}

	return
}
