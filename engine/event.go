package engine

import (
	log "github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
)

type EventHandler struct {
	engine *Engine
}

func NewEventHandler(engine *Engine) *EventHandler {
	return &EventHandler{engine}
}

func (self *EventHandler) HandleEventRequestCreated(ev event.Event) {
	request := ev.Payload.(job.JobRequest)

	log.V(1).Infof("EventRequestCreated(%s): attempting to resolve JobRequest", request.ID.String())
	err := self.engine.ResolveRequest(&request)
	if err == nil {
		log.V(1).Infof("EventRequestCreated(%s): resolved JobRequest", request.ID.String())
	} else {
		log.V(1).Infof("EventRequestCreated(%s): failed to resolve JobRequest: %s", request.ID.String(), err.Error())
	}
}

func (self *EventHandler) HandleEventJobCreated(ev event.Event) {
	j := ev.Payload.(job.Job)

	log.V(1).Infof("EventJobCreated(%s): publishing JobOffer", j.Name)
	self.engine.OfferJob(j)
}

func (self *EventHandler) HandleEventJobBidSubmitted(ev event.Event) {
	jb := ev.Payload.(job.JobBid)

	log.V(1).Infof("EventJobBidSubmitted(%s): attempting to schedule Job to Machine(%s)", jb.JobName, jb.MachineName)
	err := self.engine.ResolveJobOffer(jb.JobName, jb.MachineName)
	if err == nil {
		log.V(1).Infof("EventJobBidSubmitted(%s): successfully scheduled Job to Machine(%s)", jb.JobName, jb.MachineName)
	} else {
		log.V(1).Infof("EventJobBidSubmitted(%s): failed to schedule Job to Machine(%s): %s", jb.JobName, jb.MachineName, err.Error())
	}
}

func (self *EventHandler) HandleEventMachineRemoved(ev event.Event) {
	machName := ev.Payload.(string)
	jobs := self.engine.GetJobsScheduledToMachine(machName)

	for _, j := range jobs {
		log.V(1).Infof("EventMachineRemoved(%s): stopping Job(%s)", machName, j.Name)
		self.engine.StopJob(j.Name)
	}

	for _, j := range jobs {
		log.V(1).Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machName, j.Name)
		self.engine.OfferJob(j)
	}
}
