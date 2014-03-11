package engine

import (
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

type EventHandler struct {
	engine *Engine
}

func NewEventHandler(engine *Engine) *EventHandler {
	return &EventHandler{engine}
}

func (self *EventHandler) HandleEventJobCreated(ev event.Event) {
	j := ev.Payload.(job.Job)

	log.V(1).Infof("EventJobCreated(%s): publishing JobOffer", j.Name)
	self.engine.OfferJob(j)
}

func (self *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	machineState := ev.Payload.(machine.MachineState)
	self.engine.clust.jobScheduled(jobName, &machineState)
}

func (self *EventHandler) HandleEventJobStopped(ev event.Event) {
	jobName := ev.Payload.(string)
	self.engine.clust.jobStopped(jobName)
}

func (self *EventHandler) HandleEventJobBidSubmitted(ev event.Event) {
	jb := ev.Payload.(job.JobBid)

	log.V(1).Infof("EventJobBidSubmitted(%s): attempting to schedule Job to Machine(%s)", jb.JobName, jb.MachineBootId)
	err := self.engine.ResolveJobOffer(jb.JobName, jb.MachineBootId)
	if err == nil {
		log.V(1).Infof("EventJobBidSubmitted(%s): successfully scheduled Job to Machine(%s)", jb.JobName, jb.MachineBootId)
	} else {
		log.V(1).Infof("EventJobBidSubmitted(%s): failed to schedule Job to Machine(%s)", jb.JobName, jb.MachineBootId)
	}
}

func (self *EventHandler) HandleEventMachineRemoved(ev event.Event) {
	machBootId := ev.Payload.(string)
	mutex := self.engine.LockMachine(machBootId)
	if mutex == nil {
		log.V(2).Infof("EventMachineRemoved(%s): failed to lock Machine, ignoring event", machBootId)
		return
	}
	defer mutex.Unlock()

	jobs := self.engine.GetJobsScheduledToMachine(machBootId)

	for _, j := range jobs {
		log.V(1).Infof("EventMachineRemoved(%s): unscheduling Job(%s)", machBootId, j.Name)
		self.engine.RemoveJobState(j.Name)
		self.engine.UnscheduleJob(j.Name)
	}

	for _, j := range jobs {
		log.V(1).Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machBootId, j.Name)
		self.engine.OfferJob(j)
	}
}
