package engine

import (
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type EventHandler struct {
	engine *Engine
}

func NewEventHandler(engine *Engine) *EventHandler {
	return &EventHandler{engine}
}

func (self *EventHandler) HandleCommandLoadJob(ev event.Event) {
	jobName := ev.Payload.(string)

	j := self.engine.registry.GetJob(jobName)
	if j == nil {
		log.Infof("CommandLoadJob(%s): asked to offer job that could not be found")
		return
	}

	log.Infof("CommandLoadJob(%s): publishing JobOffer", jobName)
	self.engine.OfferJob(*j)
}

func (self *EventHandler) HandleCommandUnloadJob(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)

	if target != "" {
		log.Infof("CommandUnloadJob(%s): clearing scheduling decision", jobName)
		self.engine.registry.ClearJobTarget(jobName, target)
	}
}

func (self *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)
	log.V(1).Infof("EventJobScheduled(%s): updating cluster", jobName)
	self.engine.clust.jobScheduled(jobName, target)
}

// EventJobUnscheduled is triggered when a scheduling decision has been
// rejected, or is now unfulfillable due to changes in the cluster.
// Attempt to reschedule the job if it is in a non-inactive state.
func (self *EventHandler) HandleEventJobUnscheduled(ev event.Event) {
	jobName := ev.Payload.(string)

	ts := self.engine.registry.GetJobTargetState(jobName)
	if ts == nil || *ts == job.JobStateInactive {
		return
	}

	j := self.engine.registry.GetJob(jobName)
	if j == nil {
		log.Errorf("EventJobUnscheduled(%s): unable to re-offer Job, as it could not be found in the Registry", jobName)
		return
	}

	log.Infof("EventJobUnscheduled(%s): publishing JobOffer", jobName)
	self.engine.OfferJob(*j)
}

func (self *EventHandler) HandleCommandStopJob(ev event.Event) {
	jobName := ev.Payload.(string)
	log.V(1).Infof("EventJobStopped(%s): updating cluster", jobName)
	self.engine.clust.jobStopped(jobName)
}

func (self *EventHandler) HandleEventJobBidSubmitted(ev event.Event) {
	jb := ev.Payload.(job.JobBid)

	err := self.engine.ResolveJobOffer(jb.JobName, jb.MachineID)
	if err == nil {
		log.Infof("EventJobBidSubmitted(%s): successfully scheduled Job to Machine(%s)", jb.JobName, jb.MachineID)
	} else {
		log.Infof("EventJobBidSubmitted(%s): failed to schedule Job to Machine(%s)", jb.JobName, jb.MachineID)
	}
}

func (self *EventHandler) HandleEventMachineCreated(ev event.Event) {
	machineState := ev.Payload.(machine.MachineState)
	log.V(1).Infof("EventMachineCreated(%s): updating cluster", machineState.ID)
	self.engine.clust.machineCreated(machineState.ID)
}

func (self *EventHandler) HandleEventMachineRemoved(ev event.Event) {
	machID := ev.Payload.(string)
	mutex := self.engine.LockMachine(machID)
	if mutex == nil {
		log.V(1).Infof("EventMachineRemoved(%s): failed to lock Machine, ignoring event", machID)
		return
	}
	defer mutex.Unlock()

	jobs := self.engine.GetJobsScheduledToMachine(machID)

	for _, j := range jobs {
		log.Infof("EventMachineRemoved(%s): unscheduling Job(%s)", machID, j.Name)
		self.engine.registry.ClearJobTarget(j.Name, machID)
		self.engine.RemoveUnitState(j.Name)
	}

	for _, j := range jobs {
		log.Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machID, j.Name)
		self.engine.OfferJob(j)
	}
	self.engine.clust.machineRemoved(machID)
}
