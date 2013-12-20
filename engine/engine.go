package engine

import (
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
	self.events.RegisterListener(self, self.machine)
	self.events.Open()
}

func (self *Engine) HandleEventRequestCreated(event registry.Event) {
	request := event.Payload.(job.JobRequest)
	log.V(1).Infof("EventRequestCreated(%s): attempting to claim request", request.ID.String())
	if !self.claimRequest(&request) {
		return
	}

	log.Infof("EventRequestCreated(%s): claimed JobRequest", request.ID.String())

	//watches, _ := getJobsFromRequest(&request)
	//log.Infof("EventRequestCreated(%s): persisting %d job watches", request.ID.String(), len(watches))
	//self.persistJobWatches(watches)

	log.Infof("EventRequestCreated(%s): resolving request", request.ID.String())
	self.resolveRequest(&request)
}

func (self *Engine) claimRequest(request *job.JobRequest) bool {
	return self.registry.ClaimRequest(request, self.machine, self.claimTTL)
}

func (self *Engine) resolveRequest(request *job.JobRequest) {
	self.registry.ResolveRequest(request)
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

func (self *Engine) HandleEventMachineDeleted(event registry.Event) {
	//m := event.Payload.(machine.Machine)
	//TODO reimplement?
}
