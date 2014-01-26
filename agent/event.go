package agent

import (
	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

type EventListener struct {
	agent *Agent
}

func NewEventListener(agent *Agent) *EventListener {
	return &EventListener{agent}
}

func (el *EventListener) HandleEventJobOffered(ev event.Event) {
	jo := ev.Payload.(job.JobOffer)
	log.V(1).Infof("EventJobOffered(%s): verifying ability to run Job", jo.Job.Name)

	// Everything we check against could change over time, so we track all
	// offers starting here for future bidding even if we can't bid now
	el.agent.TrackOffer(jo)

	if el.agent.AbleToRun(&jo.Job) {
		log.Infof("EventJobOffered(%s): passed all criteria, submitting JobBid", jo.Job.Name)
		el.agent.Bid(jo.Job.Name)
	}
}

func (el *EventListener) HandleEventJobScheduled(ev event.Event) {
	jobName := ev.Payload.(string)
	log.V(1).Infof("EventJobScheduled(%s): Dropping outstanding offers and bids", jobName)

	el.agent.OfferResolved(jobName)

	if ev.Context.(*machine.Machine).BootId != el.agent.Machine().BootId {
		log.V(1).Infof("EventJobScheduled(%s): Job not scheduled to this Agent, checking unbade offers", jobName)
		eh.agent.BidForPossibleJobs()
		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Job scheduled to this Agent", jobName)

	j := el.agent.FetchJob(jobName)
	if j == nil {
		log.Errorf("EventJobScheduled(%s): Failed to fetch Job")
		return
	}

	if !el.agent.AbleToRun(j) {
		log.V(1).Infof("EventJobScheduled(%s): Unable to run scheduled Job", jobName)

		// FIXME: the listener should not talk directly to the registry
		el.agent.CancelJob(jobName)

		return
	}

	log.V(1).Infof("EventJobScheduled(%s): Starting Job", j.Name)
	el.agent.StartJob(j)

	log.V(1).Infof("EventJobScheduled(%s): Bidding for all possible peers of Job")
	el.agent.BidForPossiblePeers(jobName)
}

func (el *EventListener) HandleEventJobCancelled(ev event.Event) {
	//TODO(bcwaldon): We should check the context of the event before
	// making any changes to local systemd or the registry

	jobName := ev.Payload.(string)
	log.Infof("EventJobCancelled(%s): stopping Job", jobName)
	j := job.NewJob(jobName, nil, nil)
	el.agent.StopJob(j)

	log.V(1).Infof("EventJobCancelled(%s): revisiting unresolved JobOffers", jobName)
	el.agent.BidForPossibleJobs()
}

func (el *EventListener) HandleEventJobStateUpdated(ev event.Event) {
	jobName := ev.Context.(string)
	state := ev.Payload.(*job.JobState)
	j := job.NewJob(jobName, state, nil)

	if state == nil {
		log.V(1).Infof("EventJobStateUpdated(%s): received nil JobState object", jobName)
	} else {
		log.V(1).Infof("EventJobStateUpdated(%s): pushing state (loadState=%s, activeState=%s, subState=%s) to Registry", jobName, state.LoadState, state.ActiveState, state.SubState)

		// FIXME: This should probably be set in the underlying event-generation code
		state.Machine = el.agent.Machine()
	}

	el.agent.ReportJobState(j)
}
