package engine

import (
	"github.com/coreos/fleet/control"
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

func (self *EventHandler) HandleEventJobCreated(ev event.Event) {
	j := ev.Payload.(job.Job)

	log.V(1).Infof("EventJobCreated(%s): publishing JobOffer", j.Name)
	self.engine.OfferJob(j)
}

func (self *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	machineState := ev.Context.(machine.MachineState)
	log.V(1).Infof("EventJobScheduled(%s): updating job control", jobName)

	job := self.engine.registry.GetJob(jobName)
	spec := control.JobSpecFrom(job)
	self.engine.jobControl.JobScheduled(jobName, machineState.BootID, spec)
}

func (self *EventHandler) HandleEventJobStopped(ev event.Event) {
	jobName := ev.Payload.(string)
	machineState := ev.Context.(machine.MachineState)
	log.V(1).Infof("EventJobStopped(%s): updating job control", jobName)

	job := self.engine.registry.GetJob(jobName)
	spec := control.JobSpecFrom(job)
	self.engine.jobControl.JobDowned(jobName, machineState.BootID, spec)
}

func (self *EventHandler) HandleEventJobBidSubmitted(ev event.Event) {
	jb := ev.Payload.(job.JobBid)

	log.V(1).Infof("EventJobBidSubmitted(%s): attempting to schedule Job to Machine(%s)", jb.JobName, jb.MachineBootID)
	err := self.engine.ResolveJobOffer(jb.JobName, jb.MachineBootID)
	if err == nil {
		log.V(1).Infof("EventJobBidSubmitted(%s): successfully scheduled Job to Machine(%s)", jb.JobName, jb.MachineBootID)
	} else {
		log.V(1).Infof("EventJobBidSubmitted(%s): failed to schedule Job to Machine(%s)", jb.JobName, jb.MachineBootID)
	}
}

func (self *EventHandler) HandleEventMachineCreated(ev event.Event) {
	machineState := ev.Payload.(machine.MachineState)
	log.V(1).Infof("EventMachineCreated(%s): updating job control", machineState.BootID)
	self.engine.jobControl.HostUp(machineState.BootID)
}

func (self *EventHandler) HandleEventMachineRemoved(ev event.Event) {
	machBootID := ev.Payload.(string)
	mutex := self.engine.LockMachine(machBootID)
	if mutex == nil {
		log.V(2).Infof("EventMachineRemoved(%s): failed to lock Machine, ignoring event", machBootID)
		return
	}
	defer mutex.Unlock()

	jobs := self.engine.GetJobsScheduledToMachine(machBootID)

	for _, j := range jobs {
		log.V(1).Infof("EventMachineRemoved(%s): unscheduling Job(%s)", machBootID, j.Name)
		self.engine.RemoveJobState(j.Name)
		self.engine.UnscheduleJob(j.Name)
	}

	for _, j := range jobs {
		log.V(1).Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machBootID, j.Name)
		self.engine.OfferJob(j)
	}
	self.engine.jobControl.HostDown(machBootID)
}
