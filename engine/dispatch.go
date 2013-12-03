package engine

import (
	"fmt"
	"log"
	"time"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

const (
	DefaultClaimTTL = "5s"
)

type Dispatcher struct {
	registry  *registry.Registry
	events    *registry.EventStream
	scheduler *Scheduler
	machine   *machine.Machine
	claimTTL  time.Duration
	watches   []job.JobWatch
	machines  map[string]machine.Machine
}

func NewDispatcher(registry *registry.Registry, events *registry.EventStream, scheduler *Scheduler, m *machine.Machine) *Dispatcher {
	claimTTL, _ := time.ParseDuration(DefaultClaimTTL)
	return &Dispatcher{registry, events, scheduler, m, claimTTL, make([]job.JobWatch, 0), make(map[string]machine.Machine, 0)}
}

func (self *Dispatcher) Listen() {
	self.startEventListeners()

	for _, m := range self.registry.GetActiveMachines() {
		self.machines[m.BootId] = m
	}
}

func (self *Dispatcher) startEventListeners() {
	eventchan := make(chan registry.Event)
	self.events.RegisterGlobalEventListener(eventchan)

	handlers := map[int]func(registry.Event){
		registry.EventJobWatchCreated: self.handleEventJobWatchCreated,
		registry.EventMachineCreated:  self.handleEventMachineCreated,
		registry.EventMachineUpdated:  self.handleEventMachineUpdated,
		registry.EventMachineDeleted:  self.handleEventMachineDeleted,
		registry.EventRequestCreated:  self.handleEventRequestCreated,
	}

	for true {
		event := <-eventchan
		log.Printf("Event received: Type=%d", event.Type)

		log.Printf("Event handler begin")
		handlers[event.Type](event)
		log.Printf("Event handler complete")
	}
}

func (self *Dispatcher) handleEventRequestCreated(event registry.Event) {
	request := event.Payload.(job.JobRequest)
	log.Printf("EventRequestCreated(%s): attempting to claim request", request.ID.String())
	if !self.claimRequest(&request) {
		return
	}

	log.Printf("EventRequestCreated(%s): claimed request", request.ID.String())

	watches, _ := getJobWatchesFromRequest(&request)
	log.Printf("EventRequestCreated(%s): persisting %d job watches", request.ID.String(), len(watches))
	self.persistJobWatches(watches)

	log.Printf("EventRequestCreated(%s): resolving request", request.ID.String())
	self.resolveRequest(&request)
}

func getJobWatchesFromRequest(req *job.JobRequest) ([]job.JobWatch, error) {
	var count int
	if req.IsFlagSet(job.RequestAllMachines) {
		count = -1
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
	return self.registry.AcquireLock(request.ID.String(), self.machine.BootId, self.claimTTL)
}

func (self *Dispatcher) resolveRequest(request *job.JobRequest) {
	self.registry.ResolveRequest(request)
}

func (self *Dispatcher) submitSchedule(schedule Schedule) {
	for j, m := range schedule {
		self.registry.ScheduleMachineJob(&j, m)
	}
}

func (self *Dispatcher) persistJobWatches(watches []job.JobWatch) {
	for _, jw := range watches {
		self.registry.AddJobWatch(&jw)
	}
}

func (self *Dispatcher) handleEventJobWatchCreated(event registry.Event) {
	watch := event.Payload.(job.JobWatch)

	if !self.registry.ClaimJobWatch(&watch, self.machine) {
		log.Printf("EventJobCreated(%s): failed to claim job, discarding event", watch.Payload.Name)
		return
	}

	self.watches = append(self.watches, watch)
	sched := NewSchedule()

	if watch.Count == -1 {
		for _, m := range self.machines {
			name := fmt.Sprintf("%s.%s", m.BootId, watch.Payload.Name)
			j, _ := job.NewJob(name, nil, watch.Payload)
			log.Printf("EventJobCreated(%s): adding to schedule job=%s machine=%s", watch.Payload.Name, name, m.BootId)
			sched.Add(*j, m)
		}
	} else {
		for i := 0; i < watch.Count; i++ {
			m := pickRandomMachine(self.machines)
			name := fmt.Sprintf("%s.%s", m.BootId, watch.Payload.Name)
			j, _ := job.NewJob(name, nil, watch.Payload)
			log.Printf("EventJobCreated(%s): adding to schedule job=%s machine=%s", watch.Payload.Name, name, m.BootId)
			sched.Add(*j, *m)
		}
	}

	if len(sched) > 0 {
		log.Printf("EventJobCreated(%s): submitting schedule", watch.Payload.Name)
		self.submitSchedule(sched)
	} else {
		log.Printf("EventJobCreated(%s): no schedule changes made", watch.Payload.Name)
	}
}

func (self *Dispatcher) handleEventMachineCreated(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.Printf("EventMachineCreated(%s): event received", m.BootId)
	self.machines[m.BootId] = m
	log.Printf("EventMachineCreated(%s): updating dispatcher's machine list", m.BootId)

	sched := NewSchedule()
	for _, watch := range self.watches {
		if watch.Count == -1 {
			name := fmt.Sprintf("%s.%s", m.BootId, watch.Payload.Name)
			j, _ := job.NewJob(name, nil, watch.Payload)
			log.Printf("EventMachineCreated(%s): adding to schedule job=%s machine=%s", m.BootId, name, m.BootId)
			sched.Add(*j, m)
		}
	}

	if len(sched) > 0 {
		log.Printf("EventMachineCreated(%s): submitting schedule", m.BootId)
		self.submitSchedule(sched)
	} else {
		log.Printf("EventMachineCreated(%s): no schedule changes made", m.BootId)
	}

	log.Printf("EventMachineCreated(%s): event handler complete", m.BootId)
}

func (self *Dispatcher) handleEventMachineUpdated(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.Printf("EventMachineUpdated(%s): event received", m.BootId)
	log.Printf("EventMachineUpdated(%s): updating dispatcher's machine list", m.BootId)
	self.machines[m.BootId] = m
	log.Printf("EventMachineUpdated(%s): event handler complete", m.BootId)
}

func (self *Dispatcher) handleEventMachineDeleted(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.Printf("EventMachineDeleted(%s): removing machine from dispatcher's machine list", m.BootId)
	delete(self.machines, m.BootId)
}
