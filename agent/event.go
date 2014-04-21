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

	if !jo.OfferedTo(eh.agent.Machine().State().BootID) {
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
		log.V(1).Infof("EventJobOffered(%s): not offered to this machine", jo.Job.Name)
	}
}

func (eh *EventHandler) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)
	log.V(1).Infof("EventJobScheduled(%s): Dropping outstanding offers and bids", jobName)

	eh.agent.OfferResolved(jobName)

	if target != eh.agent.Machine().State().BootID {
		log.V(1).Infof("EventJobScheduled(%s): Job not scheduled to this Agent, purging related data from cache", jobName)
		eh.agent.ForgetJob(jobName)

		log.V(1).Infof("EventJobScheduled(%s): Checking outstanding job offers", jobName)
		eh.agent.BidForPossibleJobs()
		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Job scheduled to this Agent", jobName)

	j := eh.agent.FetchJob(jobName)
	if j == nil {
		log.Errorf("EventJobScheduled(%s): Failed to fetch Job", jobName)
		return
	}

	if !eh.agent.VerifyJob(j) {
		log.Errorf("EventJobScheduled(%s): Failed to verify job", j.Name)
		return
	}

	if !eh.agent.AbleToRun(j) {
		log.V(1).Infof("EventJobScheduled(%s): Unable to run scheduled Job, unscheduling.", jobName)
		eh.agent.registry.ClearJobTarget(jobName, target)
		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Loading Job", j.Name)
	eh.agent.LoadJob(j)

	log.V(1).Infof("EventJobScheduled(%s): Bidding for all possible peers of Job", j.Name)
	eh.agent.BidForPossiblePeers(j.Name)

	ts := eh.agent.registry.GetJobTargetState(j.Name)
	if ts == nil || *ts != job.JobStateLaunched {
		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Starting Job", j.Name)
	eh.agent.StartJob(j.Name)
}

func (eh *EventHandler) HandleCommandStartJob(ev event.Event) {
	if ev.Context.(string) != eh.agent.Machine().State().BootID {
		return
	}

	jobName := ev.Payload.(string)
	log.Infof("CommandStartJob(%s): starting corresponding unit", jobName)
	eh.agent.StartJob(jobName)
}

func (eh *EventHandler) HandleCommandStopJob(ev event.Event) {
	if ev.Context.(string) != eh.agent.Machine().State().BootID {
		return
	}

	jobName := ev.Payload.(string)
	log.Infof("CommandStopJob(%s): stopping corresponding unit", jobName)
	eh.agent.StopJob(jobName)
}

func (eh *EventHandler) HandleEventJobUnscheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	target := ev.Context.(string)

	if target != eh.agent.Machine().State().BootID {
		log.V(1).Infof("EventJobUnscheduled(%s): not scheduled here, ignoring ", jobName)
		return
	}

	log.Infof("EventJobUnscheduled(%s): unloading job", jobName)
	eh.agent.UnloadJob(jobName)
}

func (eh *EventHandler) HandleEventJobDestroyed(ev event.Event) {
	jobName := ev.Payload.(string)

	log.Infof("EventJobDestroyed(%s): unloading corresponding unit", jobName)
	eh.agent.UnloadJob(jobName)
}

func (eh *EventHandler) HandleEventPayloadStateUpdated(ev event.Event) {
	jobName := ev.Context.(string)
	state := ev.Payload.(*job.PayloadState)

	if state == nil {
		log.V(1).Infof("EventPayloadStateUpdated(%s): received nil PayloadState object", jobName)
		state, _ = eh.agent.systemd.GetPayloadState(jobName)
	}

	log.V(1).Infof("EventPayloadStateUpdated(%s): pushing state (loadState=%s, activeState=%s, subState=%s) to Registry", jobName, state.LoadState, state.ActiveState, state.SubState)

	// FIXME: This should probably be set in the underlying event-generation code
	ms := eh.agent.Machine().State()
	state.MachineState = &ms

	eh.agent.ReportPayloadState(jobName, state)
}

func (eh *EventHandler) HandleEventMachineCreated(ev event.Event) {
	mach := ev.Payload.(machine.MachineState)
	if mach.BootID != eh.agent.Machine().State().BootID {
		log.V(1).Infof("EventMachineCreated(%s): references other Machine, discarding event", mach.BootID)
	}

	for _, jo := range eh.agent.UnresolvedJobOffers() {
		log.V(1).Infof("EventMachineCreated(%s): verifying ability to run Job(%s)", mach.BootID, jo.Job.Name)

		// Everything we check against could change over time, so we track all
		// offers starting here for future bidding even if we can't bid now
		eh.agent.TrackOffer(jo)
	}

	eh.agent.BidForPossibleJobs()
}
