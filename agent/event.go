package agent

import (
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

type EventHandler struct {
	agent *Agent
}

func NewEventHandler(agent *Agent) *EventHandler {
	return &EventHandler{agent}
}

func (eh *EventHandler) HandleEventJobOffered(ev event.Event) {
	jo := ev.Payload.(job.JobOffer)

	if !jo.OfferedTo(eh.agent.Machine.State().ID) {
		log.V(1).Infof("EventJobOffered(%s): not offered to this machine, ignoring", jo.Job.Name)
		return
	}

	log.Infof("EventJobOffered(%s): deciding whether to bid or not", jo.Job.Name)
	eh.agent.MaybeBid(jo)
}

func (eh *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)

	log.Infof("EventJobScheduled(%s): Job(%s) scheduled to Machine(%s), deciding what to do", jobName, jobName, target)
	eh.agent.JobScheduled(jobName, target)
}

func (eh *EventHandler) HandleCommandStartJob(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)

	if target != eh.agent.Machine.State().ID {
		log.V(1).Infof("CommandStartJob(%s): scheduled elsewhere, ignoring", jobName)
		return
	}

	log.Infof("CommandStartJob(%s): instructing Agent to start Job", jobName)
	eh.agent.StartJob(jobName)
}

func (eh *EventHandler) HandleCommandStopJob(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)

	if target != eh.agent.Machine.State().ID {
		log.V(1).Infof("CommandStopJob(%s): scheduled elsewhere, ignoring", jobName)
		return
	}

	log.Infof("CommandStopJob(%s): instructing Agent to stop Job", jobName)
	eh.agent.StopJob(jobName)
}

func (eh *EventHandler) HandleEventJobUnscheduled(ev event.Event) {
	eh.unloadJobEvent(ev)
}

func (eh *EventHandler) HandleEventJobDestroyed(ev event.Event) {
	eh.unloadJobEvent(ev)
}

// unloadJobEvent handles an event by unloading the job to which it
// refers. The event's payload must be a string representing the
// name of a Job. If the Job is not scheduled locally, it will be
// ignored.
func (eh *EventHandler) unloadJobEvent(ev event.Event) {
	jobName := ev.Payload.(string)

	log.Infof("%s(%s): Job(%s) unscheduled, deciding what to do", ev.Type, jobName, jobName)
	eh.agent.JobUnscheduled(jobName)
}

func (eh *EventHandler) HandleEventUnitStateUpdated(ev event.Event) {
	jobName := ev.Context.(string)
	state := ev.Payload.(*unit.UnitState)

	if state == nil {
		log.V(1).Infof("EventUnitStateUpdated(%s): received nil UnitState object, ignoring", jobName)
		return
	}

	log.Infof("EventUnitStateUpdated(%s): reporting state to Registry")
	eh.agent.ReportUnitState(jobName, state)
}
