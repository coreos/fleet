package engine

import (
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

type EventHandler struct {
	engine *Engine
}

func NewEventHandler(engine *Engine) *EventHandler {
	return &EventHandler{engine}
}

func (eh *EventHandler) HandleCommandLoadJob(ev event.Event) {
	jobName := ev.Payload.(string)

	j, _ := eh.engine.registry.GetJob(jobName)
	if j == nil {
		log.Infof("CommandLoadJob(%s): asked to offer job that could not be found")
		return
	}

	log.Infof("CommandLoadJob(%s): publishing JobOffer", jobName)
	eh.engine.OfferJob(*j)
}

func (eh *EventHandler) HandleCommandUnloadJob(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)

	if target != "" {
		log.Infof("CommandUnloadJob(%s): clearing scheduling decision", jobName)
		eh.engine.registry.ClearJobTarget(jobName, target)
	}
}

func (eh *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)
	log.V(1).Infof("EventJobScheduled(%s): updating cluster", jobName)
	eh.engine.clust.jobScheduled(jobName, target)
}

// EventJobUnscheduled is triggered when a scheduling decision has been
// rejected, or is now unfulfillable due to changes in the cluster.
// Attempt to reschedule the job if it is in a non-inactive state.
func (eh *EventHandler) HandleEventJobUnscheduled(ev event.Event) {
	jobName := ev.Payload.(string)

	ts, _ := eh.engine.registry.GetJobTargetState(jobName)
	if ts == nil || *ts == job.JobStateInactive {
		return
	}

	j, _ := eh.engine.registry.GetJob(jobName)
	if j == nil {
		log.Errorf("EventJobUnscheduled(%s): unable to re-offer Job, as it could not be found in the Registry", jobName)
		return
	}

	log.Infof("EventJobUnscheduled(%s): publishing JobOffer", jobName)
	eh.engine.OfferJob(*j)
}

func (eh *EventHandler) HandleCommandStopJob(ev event.Event) {
	jobName := ev.Payload.(string)
	log.V(1).Infof("EventJobStopped(%s): updating cluster", jobName)
	eh.engine.clust.jobStopped(jobName)
}

func (eh *EventHandler) HandleEventJobBidSubmitted(ev event.Event) {
	jb := ev.Payload.(job.JobBid)

	err := eh.engine.ResolveJobOffer(jb.JobName, jb.MachineID)
	if err == nil {
		log.Infof("EventJobBidSubmitted(%s): successfully scheduled Job to Machine(%s)", jb.JobName, jb.MachineID)
	} else {
		log.Infof("EventJobBidSubmitted(%s): failed to schedule Job to Machine(%s)", jb.JobName, jb.MachineID)
	}
}

func (eh *EventHandler) HandleEventMachineCreated(ev event.Event) {
	machineState := ev.Payload.(machine.MachineState)
	log.V(1).Infof("EventMachineCreated(%s): updating cluster", machineState.ID)
	eh.engine.clust.machineCreated(machineState.ID)
}

func (eh *EventHandler) HandleEventMachineRemoved(ev event.Event) {
	machID := ev.Payload.(string)
	mutex := eh.engine.registry.LockMachine(machID, eh.engine.machine.State().ID)
	if mutex == nil {
		log.V(1).Infof("EventMachineRemoved(%s): failed to lock Machine, ignoring event", machID)
		return
	}
	defer mutex.Unlock()

	jobs := getJobsScheduledToMachine(eh.engine.registry, machID)

	for _, j := range jobs {
		log.Infof("EventMachineRemoved(%s): unscheduling Job(%s)", machID, j.Name)
		eh.engine.registry.ClearJobTarget(j.Name, machID)
		eh.engine.registry.RemoveUnitState(j.Name)
	}

	for _, j := range jobs {
		log.Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machID, j.Name)
		eh.engine.OfferJob(j)
	}
	eh.engine.clust.machineRemoved(machID)
}

func getJobsScheduledToMachine(r registry.Registry, machID string) []job.Job {
	var jobs []job.Job

	jj, _ := r.GetAllJobs()
	for _, j := range jj {
		tgt, _ := r.GetJobTarget(j.Name)
		if tgt == "" || tgt != machID {
			continue
		}
		jobs = append(jobs, j)
	}

	return jobs
}
