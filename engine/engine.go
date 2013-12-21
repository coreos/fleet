package engine

import (
	"fmt"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

const (
	DefaultRequestClaimTTL = "10s"
)

type Engine struct {
	registry  *registry.Registry
	events    *registry.EventStream
	scheduler *Scheduler
	machine   *machine.Machine
	claimTTL  time.Duration
}

func New(reg *registry.Registry, events *registry.EventStream, mach *machine.Machine) *Engine {
	scheduler := NewScheduler()
	claimTTL, _ := time.ParseDuration(DefaultRequestClaimTTL)
	return &Engine{reg, events, scheduler, mach, claimTTL}
}

func (self *Engine) Run() {
	self.events.AddListener("engine", self.machine, self)
}

func (self *Engine) HandleEventRequestCreated(event registry.Event) {
	request := event.Payload.(job.JobRequest)

	log.V(1).Infof("EventRequestCreated(%s): attempting to claim JobRequest", request.ID.String())
	if !self.claimRequest(&request) {
		return
	}

	log.Infof("EventRequestCreated(%s): claimed JobRequest", request.ID.String())

	for _, j := range getJobsFromRequest(&request) {
		log.Infof("EventRequestCreated(%s): creating Job(%s)", request.ID.String(), j.Name)
		self.registry.CreateJob(&j)
	}

	log.Infof("EventRequestCreated(%s): resolving JobRequest", request.ID.String())
	self.registry.ResolveRequest(&request)
}

func getJobsFromRequest(req *job.JobRequest) []job.Job {
	var jobs []job.Job
	for i := 1; i <= req.Count; i++ {
		for ii := 0; ii < len(req.Payloads); ii++ {
			payload := req.Payloads[ii]
			j, _ := job.NewJob(fmt.Sprintf("%d.%s", i, payload.Name), nil, &payload)
			jobs = append(jobs, *j)
		}
	}
	return jobs
}

func (self *Engine) claimRequest(req *job.JobRequest) bool {
	return self.registry.ClaimRequest(req, self.machine, self.claimTTL)
}

func (self *Engine) HandleEventJobCreated(event registry.Event) {
	j := event.Payload.(job.Job)
	log.V(1).Infof("EventJobCreated(%s): Job=%s", j.Name, j.String())

	log.V(1).Infof("EventJobCreated(%s): attempting to claim Job", j.Name)
	if !self.claimJob(j.Name) {
		log.V(1).Infof("EventJobCreated(%s): unable to claim Job", j.Name)
		return
	}

	offer := job.NewOfferFromJob(j)
	log.V(1).Infof("EventJobCreated(%s): created JobOffer(%s) with Peers(%s)", j.Name, offer.Job.Name, strings.Join(offer.Peers, ","))

	log.Infof("EventJobCreated(%s): publishing JobOffer(%s)", j.Name, offer.Job.Name)
	self.registry.CreateJobOffer(offer)
}

func (self *Engine) claimJob(jobName string) bool {
	return self.registry.ClaimJob(jobName, self.machine, self.claimTTL)
}

func (self *Engine) HandleEventJobBidSubmitted(event registry.Event) {
	jb := event.Payload.(job.JobBid)

	log.V(1).Infof("EventJobBidSubmitted(%s): attempting to claim JobOffer", jb.JobName)
	if !self.claimJobOffer(jb.JobName) {
		log.V(1).Infof("EventJobBidSubmitted(%s): could not claim JobOffer", jb.JobName)
		return
	}

	log.V(1).Infof("EventJobBidSubmitted(%s): accepted JobBid from Machine(%s), resolving JobOffer", jb.JobName, jb.MachineName)
	self.registry.ResolveJobOffer(jb.JobName)

	log.Infof("EventJobBidSubmitted(%s): instructing Machine(%s) to run Job", jb.JobName, jb.MachineName)
	self.registry.ScheduleJob(jb.JobName, jb.MachineName)
}

func (self *Engine) claimJobOffer(jobName string) bool {
	return self.registry.ClaimJobOffer(jobName, self.machine, self.claimTTL)
}

func (self *Engine) HandleEventJobStatePublished(event registry.Event) {
	//j := event.Payload.(job.Job)
	//TODO reimplement
}

func (self *Engine) HandleEventJobStateExpired(event registry.Event) {
	//j := event.Payload.(job.Job)
	//TODO reimplement
}

func (self *Engine) HandleEventMachineUpdated(event registry.Event) {
	//m := event.Payload.(machine.Machine)
	//TODO reimplement?
}

func (self *Engine) HandleEventMachineRemoved(event registry.Event) {
	machName := event.Payload.(string)
	for _, j := range self.registry.GetAllJobs() {
		tgt := self.registry.GetJobTarget(j.Name)
		if tgt == nil || tgt.BootId != machName {
			continue
		}

		log.V(1).Infof("EventMachineRemoved(%s): cancelling Job(%s)", machName, j.Name)
		self.registry.CancelJob(j.Name)

		offer := job.NewOfferFromJob(j)
		log.V(1).Infof("EventMachineRemoved(%s): re-publishing JobOffer(%s)", machName, offer.Job.Name)
		self.registry.CreateJobOffer(offer)
	}
}
