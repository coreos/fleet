package engine

import (
	"errors"
	"time"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

const (
	DefaultRequestClaimTTL = "4s"
)

type Engine struct {
	registry *registry.Registry
	events   *event.EventBus
	machine  *machine.Machine
	claimTTL time.Duration

	stop chan bool
}

func New(reg *registry.Registry, events *event.EventBus, mach *machine.Machine) *Engine {
	claimTTL, _ := time.ParseDuration(DefaultRequestClaimTTL)
	return &Engine{reg, events, mach, claimTTL, nil}
}

func (self *Engine) Run() {
	self.stop = make(chan bool)

	handler := NewEventHandler(self)
	self.events.AddListener("engine", self.machine, handler)

	// Block until we receive a stop signal
	<-self.stop

	self.events.RemoveListener("engine", self.machine)
}

func (self *Engine) Stop() {
	log.V(1).Info("Stopping Engine")
	close(self.stop)
}

func (self *Engine) GetJobsScheduledToMachine(machName string) []job.Job {
	var jobs []job.Job

	for _, j := range self.registry.GetAllJobs() {
		tgt := self.registry.GetJobTarget(j.Name)
		if tgt == nil || tgt.BootId != machName {
			continue
		}
		jobs = append(jobs, j)
	}

	return jobs
}

func (self *Engine) CancelJob(jobName string) {
	self.registry.CancelJob(jobName)
	log.Info("Cancelled Job(%s)", jobName)
}

func (self *Engine) ResolveRequest(request *job.JobRequest) error {
	log.V(2).Infof("Attempting to claim JobRequest(%s)", request.ID.String())
	if !self.claimRequest(request) {
		log.V(2).Infof("Could not claim JobRequest(%s)", request.ID.String())
		return errors.New("Could not claim JobRequest")
	}

	log.V(1).Infof("Claimed JobRequest(%s)", request.ID.String())

	for _, j := range getJobsFromRequest(request) {
		log.V(2).Infof("Creating Job(%s) from JobRequest(%s)", j.Name, request.ID.String())
		self.registry.CreateJob(&j)
		log.Infof("Created Job(%s) from JobRequest(%s)", j.Name, request.ID.String())
	}

	log.V(2).Infof("Resolving JobRequest(%s)", request.ID.String())
	self.registry.ResolveRequest(request)
	log.V(1).Infof("Resolved JobRequest(%s)", request.ID.String())

	return nil
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

func (self *Engine) OfferJob(j job.Job) error {
	log.V(2).Infof("Attempting to claim Job(%s)", j.Name)
	if !self.claimJob(j.Name) {
		log.V(1).Infof("Could not claim Job(%s)", j.Name)
		return errors.New("Could not claim Job")
	}

	log.V(1).Infof("Claimed Job", j.Name)

	offer := job.NewOfferFromJob(j)

	log.V(2).Infof("Publishing JobOffer(%s)", offer.Job.Name)
	self.registry.CreateJobOffer(offer)
	log.Infof("Published JobOffer(%s)", offer.Job.Name)

	return nil
}

func (self *Engine) ResolveJobOffer(jobName string, machName string) error {
	log.V(2).Infof("Attempting to claim JobOffer(%s)", jobName)
	if !self.claimJobOffer(jobName) {
		log.V(2).Infof("Could not claim JobOffer(%s)", jobName)
		return errors.New("Could not claim JobOffer")
	}

	log.V(2).Infof("Claimed JobOffer", jobName)

	log.V(2).Infof("Resolving JobOffer(%s), scheduling to Machine(%s)", jobName, machName)
	self.registry.ResolveJobOffer(jobName)
	self.registry.ScheduleJob(jobName, machName)
	log.Infof("Scheduled Job(%s) to Machine(%s)", jobName, machName)

	return nil
}

func (self *Engine) claimJobOffer(jobName string) bool {
	return self.registry.ClaimJobOffer(jobName, self.machine, self.claimTTL)
}

func (self *Engine) claimJob(jobName string) bool {
	return self.registry.ClaimJob(jobName, self.machine, self.claimTTL)
}

func (self *Engine) claimRequest(req *job.JobRequest) bool {
	return self.registry.ClaimRequest(req, self.machine, self.claimTTL)
}
