package engine

import (
	"errors"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

const (
	DefaultClaimTTL = "4s"
)

type Engine struct {
	registry	*registry.Registry
	events		*event.EventBus
	machine		*machine.Machine
	claimTTL	time.Duration

	stop	chan bool
}

func New(reg *registry.Registry, events *event.EventBus, mach *machine.Machine) *Engine {
	claimTTL, _ := time.ParseDuration(DefaultClaimTTL)
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

func (self *Engine) UnscheduleJob(jobName string) {
	self.registry.UnscheduleJob(jobName)
	log.Infof("Unscheduled Job(%s)", jobName)
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
