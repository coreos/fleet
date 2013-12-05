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
	ScheduleAllMachines = -1
)

type Dispatcher struct {
	registry *registry.Registry
	events   *registry.EventStream
	watcher  *JobWatcher
	machine  *machine.Machine
	claimTTL time.Duration
}

func NewDispatcher(registry *registry.Registry, events *registry.EventStream, watcher *JobWatcher, m *machine.Machine) *Dispatcher {
	claimTTL, _ := time.ParseDuration(DefaultRequestClaimTTL)
	return &Dispatcher{registry, events, watcher, m, claimTTL}
}

func (self *Dispatcher) Listen() {
	eventchan := make(chan registry.Event)
	self.events.RegisterGlobalEventListener(eventchan)

	handlers := map[int]func(registry.Event){
		registry.EventJobWatchCreated:      self.handleEventJobWatchCreated,
		registry.EventJobWatchClaimExpired: self.handleEventJobWatchCreated,
		registry.EventJobWatchDeleted:      self.handleEventJobWatchDeleted,
		registry.EventMachineCreated:       self.handleEventMachineCreated,
		registry.EventMachineUpdated:       self.handleEventMachineUpdated,
		registry.EventMachineDeleted:       self.handleEventMachineDeleted,
		registry.EventRequestCreated:       self.handleEventRequestCreated,
	}

	for true {
		event := <-eventchan
		log.V(1).Infof("Event received: Type=%d", event.Type)
		handler := handlers[event.Type]
		log.V(1).Infof("Event handler begin")
		handler(event)
		log.V(1).Infof("Event handler complete")
	}
}

func (self *Dispatcher) handleEventRequestCreated(event registry.Event) {
	request := event.Payload.(job.JobRequest)
	log.V(1).Infof("EventRequestCreated(%s): attempting to claim request", request.ID.String())
	if !self.claimRequest(&request) {
		return
	}

	log.Infof("EventRequestCreated(%s): claimed JobRequest", request.ID.String())

	watches, _ := getJobWatchesFromRequest(&request)
	log.Infof("EventRequestCreated(%s): persisting %d job watches", request.ID.String(), len(watches))
	self.persistJobWatches(watches)

	log.Infof("EventRequestCreated(%s): resolving request", request.ID.String())
	self.resolveRequest(&request)
}

func getJobWatchesFromRequest(req *job.JobRequest) ([]job.JobWatch, error) {
	var count int
	if req.IsFlagSet(job.RequestAllMachines) {
		count = ScheduleAllMachines
	} else {
		count = req.Count
	}

	watches := make([]job.JobWatch, 0)
	for i := 0; i < len(req.Payloads); i++ {
		// Manually create the payload variable so we get a full copy
		// of the data, not just a shallow copy.
		payload := req.Payloads[i]

		jw := job.NewJobWatch(&payload, count)
		watches = append(watches, *jw)
	}
	return watches, nil
}

func (self *Dispatcher) claimRequest(request *job.JobRequest) bool {
	return self.registry.ClaimRequest(request, self.machine, self.claimTTL)
}

func (self *Dispatcher) resolveRequest(request *job.JobRequest) {
	self.registry.ResolveRequest(request)
}

func (self *Dispatcher) persistJobWatches(watches []job.JobWatch) {
	for _, jw := range watches {
		self.registry.AddJobWatch(&jw)
	}
}

func (self *Dispatcher) handleEventJobWatchCreated(event registry.Event) {
	watch := event.Payload.(job.JobWatch)
	log.V(1).Infof("EventJobWatchCreated(%s): attempting to claim JobWatch", watch.Payload.Name)
	if ok := self.watcher.AddJobWatch(&watch); !ok {
		log.V(1).Infof("EventJobWatchCreated(%s): failed to claim job, discarding event", watch.Payload.Name)
	}
}

func (self *Dispatcher) handleEventJobWatchDeleted(event registry.Event) {
	watch := event.Payload.(job.JobWatch)
	if ok := self.watcher.RemoveJobWatch(&watch); ok {
		log.V(1).Infof("EventJobWatchDeleted(%s): removed JobWatch from watcher", watch.Payload.Name)
	} else {
		log.V(1).Infof("EventJobWatchDeleted(%s): no ownership of JobWatch, discarding event", watch.Payload.Name)
	}
}

func (self *Dispatcher) handleEventMachineCreated(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.V(1).Infof("EventMachineCreated(%s): updating JobWatcher's machine list", m.BootId)
	self.watcher.TrackMachine(&m)
}

func (self *Dispatcher) handleEventMachineUpdated(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.V(1).Infof("EventMachineUpdated(%s): updating JobWatcher's machine list", m.BootId)
	self.watcher.TrackMachine(&m)
}

func (self *Dispatcher) handleEventMachineDeleted(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.V(1).Infof("EventMachineDeleted(%s): removing machine from dispatcher's machine list", m.BootId)
	self.watcher.DropMachine(&m)
}
