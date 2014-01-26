package engine

import (
	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/job"
)

type EventHandler struct {
	engine *Engine
}

func NewEventHandler(engine *Engine) *EventHandler {
	return &EventHandler{engine}
}

func (self *EventHandler) HandleEventRequestCreated(ev event.Event) {
	request := ev.Payload.(job.JobRequest)

	log.V(1).Infof("EventRequestCreated(%s): attempting to claim JobRequest", request.ID.String())
	if !self.claimRequest(&request) {
		log.V(1).Infof("EventRequestCreated(%s): could not claim JobRequest", request.ID.String())
		return
	} else {
		log.V(1).Infof("EventRequestCreated(%s): claimed JobRequest", request.ID.String())
	}

	for _, j := range getJobsFromRequest(&request) {
		log.Infof("EventRequestCreated(%s): creating Job(%s)", request.ID.String(), j.Name)
		self.engine.Registry().CreateJob(&j)
	}

	log.Infof("EventRequestCreated(%s): resolving JobRequest", request.ID.String())
	self.engine.Registry().ResolveRequest(&request)
}

func getJobsFromRequest(req *job.JobRequest) []job.Job {
	var jobs []job.Job
	for i := 0; i < len(req.Payloads); i++ {
		payload := req.Payloads[i]
		j := job.NewJob(payload.Name, nil, &payload)
		jobs = append(jobs, *j)
	}
	return jobs
}

func (self *EventHandler) claimRequest(req *job.JobRequest) bool {
	return self.engine.Registry().ClaimRequest(req, self.engine.Machine(), self.engine.ClaimTTL())
}

func (self *EventHandler) HandleEventJobCreated(ev event.Event) {
	j := ev.Payload.(job.Job)
	log.V(1).Infof("EventJobCreated(%s): Job=%s", j.Name, j.String())

	log.V(1).Infof("EventJobCreated(%s): attempting to claim Job", j.Name)
	if !self.claimJob(j.Name) {
		log.V(1).Infof("EventJobCreated(%s): unable to claim Job", j.Name)
		return
	} else {
		log.V(1).Infof("EventJobCreated(%s): claimed Job", j.Name)
	}

	offer := job.NewOfferFromJob(j)
	log.V(1).Infof("EventJobCreated(%s): created JobOffer(%s)", j.Name, offer.Job.Name)

	log.Infof("EventJobCreated(%s): publishing JobOffer(%s)", j.Name, offer.Job.Name)
	self.engine.Registry().CreateJobOffer(offer)
}

func (self *EventHandler) claimJob(jobName string) bool {
	return self.engine.Registry().ClaimJob(jobName, self.engine.Machine(), self.engine.ClaimTTL())
}

func (self *EventHandler) HandleEventJobBidSubmitted(ev event.Event) {
	jb := ev.Payload.(job.JobBid)

	log.V(1).Infof("EventJobBidSubmitted(%s): attempting to claim JobOffer", jb.JobName)
	if !self.claimJobOffer(jb.JobName) {
		log.V(1).Infof("EventJobBidSubmitted(%s): could not claim JobOffer", jb.JobName)
		return
	} else {
		log.V(1).Infof("EventJobBidSubmitted(%s): claimed JobOffer", jb.JobName)
	}

	log.V(1).Infof("EventJobBidSubmitted(%s): accepted JobBid from Machine(%s), resolving JobOffer", jb.JobName, jb.MachineName)
	self.engine.Registry().ResolveJobOffer(jb.JobName)

	log.Infof("EventJobBidSubmitted(%s): instructing Machine(%s) to run Job", jb.JobName, jb.MachineName)
	self.engine.Registry().ScheduleJob(jb.JobName, jb.MachineName)
}

func (self *EventHandler) claimJobOffer(jobName string) bool {
	return self.engine.Registry().ClaimJobOffer(jobName, self.engine.Machine(), self.engine.ClaimTTL())
}

func (self *EventHandler) HandleEventMachineRemoved(ev event.Event) {
	machName := ev.Payload.(string)
	for _, j := range self.engine.Registry().GetAllJobs() {
		tgt := self.engine.Registry().GetJobTarget(j.Name)
		if tgt == nil || tgt.BootId != machName {
			continue
		}

		log.V(1).Infof("EventMachineRemoved(%s): cancelling Job(%s)", machName, j.Name)
		self.engine.Registry().CancelJob(j.Name)

		offer := job.NewOfferFromJob(j)
		log.V(1).Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machName, offer.Job.Name)
		self.engine.Registry().CreateJobOffer(offer)
	}
}
