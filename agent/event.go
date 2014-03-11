package agent

import (
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

type EventHandler struct {
	agent *Agent
}

func NewEventHandler(agent *Agent) *EventHandler {
	return &EventHandler{agent}
}

func (eh *EventHandler) HandleEventJobOffered(ev event.Event) {
	jo := ev.Payload.(job.JobOffer)
	log.V(1).Infof("EventJobOffered(%s): verifying ability to run Job", jo.Job.Name)

	if !jo.OfferedTo(eh.agent.Machine().State().BootId) {
		log.V(1).Infof("EventJobOffered(%s): not offered to this machine", jo.Job.Name)
		return
	}
	// Everything we check against could change over time, so we track all
	// offers starting here for future bidding even if we can't bid now
	eh.agent.TrackOffer(jo)

	if eh.agent.AbleToRun(&jo.Job) {
		log.Infof("EventJobOffered(%s): passed all criteria, submitting JobBid", jo.Job.Name)
		eh.agent.Bid(jo.Job.Name)
	} else {
		log.V(1).Infof("EventJobOffered(%s): not all criteria met, not bidding", jo.Job.Name)
	}
}

func (eh *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	log.V(1).Infof("EventJobScheduled(%s): Dropping outstanding offers and bids", jobName)

	eh.agent.OfferResolved(jobName)

	if ev.Context.(machine.MachineState).BootId != eh.agent.Machine().State().BootId {
		log.V(1).Infof("EventJobScheduled(%s): Job not scheduled to this Agent, checking unbade offers", jobName)
		eh.agent.BidForPossibleJobs()
		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Job scheduled to this Agent", jobName)

	j := eh.agent.FetchJob(jobName)
	if j == nil {
		log.Errorf("EventJobScheduled(%s): Failed to fetch Job")
		return
	}

	if !eh.agent.AbleToRun(j) {
		log.V(1).Infof("EventJobScheduled(%s): Unable to run scheduled Job, rescheduling.", jobName)
		eh.agent.RescheduleJob(j)
		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Starting Job", j.Name)
	eh.agent.StartJob(j)

	log.V(1).Infof("EventJobScheduled(%s): Bidding for all possible peers of Job", j.Name)
	eh.agent.BidForPossiblePeers(jobName)
}

func (eh *EventHandler) HandleEventJobStopped(ev event.Event) {
	//TODO(bcwaldon): We should check the context of the event before
	// making any changes to local systemd or the registry

	jobName := ev.Payload.(string)
	log.Infof("EventJobStopped(%s): stopping corresponding unit", jobName)
	eh.agent.StopJob(jobName)

	log.V(1).Infof("EventJobStopped(%s): revisiting unresolved JobOffers", jobName)
	eh.agent.BidForPossibleJobs()
}

func (eh *EventHandler) HandleEventJobStateUpdated(ev event.Event) {
	jobName := ev.Context.(string)
	state := ev.Payload.(*job.JobState)

	if state == nil {
		log.V(1).Infof("EventJobStateUpdated(%s): received nil JobState object", jobName)
	} else {
		log.V(1).Infof("EventJobStateUpdated(%s): pushing state (loadState=%s, activeState=%s, subState=%s) to Registry", jobName, state.LoadState, state.ActiveState, state.SubState)

		// FIXME: This should probably be set in the underlying event-generation code
		ms := eh.agent.Machine().State()
		state.MachineState = &ms
	}

	eh.agent.ReportJobState(jobName, state)
}

func (eh *EventHandler) HandleEventMachineCreated(ev event.Event) {
	mach := ev.Payload.(machine.MachineState)
	if mach.BootId != eh.agent.Machine().State().BootId {
		log.V(1).Infof("EventMachineCreated(%s): references other Machine, discarding event", mach.BootId)
	}

	for _, jo := range eh.agent.UnresolvedJobOffers() {
		log.V(1).Infof("EventMachineCreated(%s): verifying ability to run Job(%s)", mach.BootId, jo.Job.Name)

		// Everything we check against could change over time, so we track all
		// offers starting here for future bidding even if we can't bid now
		eh.agent.TrackOffer(jo)

		if eh.agent.AbleToRun(&jo.Job) {
			log.Infof("EventMachineCreated(%s): passed all criteria, submitting JobBid(%s)", mach.BootId, jo.Job.Name)
			eh.agent.Bid(jo.Job.Name)
		} else {
			log.V(1).Infof("EventMachineCreated(%s): not all criteria met, not bidding for Job(%s)", mach.BootId, jo.Job.Name)
		}
	}
}
