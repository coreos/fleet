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
	jobs      map[string]job.Job
	machines  map[string]machine.Machine
}

func NewDispatcher(registry *registry.Registry, events *registry.EventStream, scheduler *Scheduler, m *machine.Machine) *Dispatcher {
	claimTTL, _ := time.ParseDuration(DefaultClaimTTL)
	return &Dispatcher{registry, events, scheduler, m, claimTTL, make(map[string]job.Job, 0), make(map[string]machine.Machine, 0)}
}

func (self *Dispatcher) Listen() {
	self.jobs = self.registry.GetClusterJobs()
	for _, m := range self.registry.GetActiveMachines() {
		self.machines[m.BootId] = m
	}

	//FIXME: If a machine is added before the listeners are set up but after
	// we call GetActiveMachines, it won't make it into self.machines

	self.startEventListeners()
}

func (self *Dispatcher) startEventListeners() {
	eventchan := make(chan registry.Event)
	self.events.RegisterGlobalEventListener(eventchan)

	handlers := map[int]func(registry.Event){
		registry.EventJobCreated:     self.handleEventJobCreated,
		registry.EventMachineCreated: self.handleEventMachineCreated,
		registry.EventMachineDeleted: self.handleEventMachineDeleted,
		registry.EventRequestCreated: self.handleEventRequestCreated,
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

	jobs, err := getJobsFromRequest(&request)
	if err != nil {
		log.Printf("EventRequestCreated(%s): unable to determine jobs from request: %s", request.ID.String(), err)
		return
	}

	if request.IsFlagSet(job.RequestAllMachines) {
		log.Printf("EventRequestCreated(%s): writing cluster jobs to registry", request.ID.String())
		self.persistClusterJobs(jobs)
	} else {
		log.Printf("EventRequestCreated(%s): building schedule from request", request.ID.String())
		schedule, _ := self.scheduler.BuildSchedule(jobs, self.machines, self.registry)
		log.Printf("EventRequestCreated(%s): submitting schedule built from request: %s", request.ID.String(), schedule.String())
		self.submitSchedule(schedule)
	}

	log.Printf("EventRequestCreated(%s): resolving request", request.ID.String())
	self.resolveRequest(&request)
}

func getJobsFromRequest(req *job.JobRequest) ([]job.Job, error) {
	jobs := make([]job.Job, 0)
	for i := 0; i < len(req.Payloads); i++ {
		// Manually create the payload variable so we get a full copy
		// of the data, not just a shallow copy.
		payload := req.Payloads[i]

		j, err := job.NewJob(payload.Name, nil, &payload)
		if err != nil {
			return nil, err
		} else {
			jobs = append(jobs, *j)
		}
	}
	return jobs, nil
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

func (self *Dispatcher) persistClusterJobs(jobs []job.Job) {
	for _, j := range jobs {
		self.registry.ScheduleClusterJob(&j)
	}
}

func (self *Dispatcher) handleEventJobCreated(event registry.Event) {
	clusterJob := event.Payload.(job.Job)
	self.jobs[clusterJob.Name] = clusterJob
	log.Printf("EventJobCreated(%s): added cluster Job to dispatcher's active list", clusterJob.Name)

	sched := NewSchedule()
	for _, m := range self.machines {
		name := fmt.Sprintf("%s.%s", m.BootId, clusterJob.Name)
		log.Printf("EventJobCreated(%s): adding to schedule job=%s machine=%s", clusterJob.Name, name, m.BootId)

		j, _ := job.NewJob(name, nil, clusterJob.Payload)
		sched.Add(*j, m)
	}
	if len(sched) > 0 {
		log.Printf("EventJobCreated(%s): submitting schedule", clusterJob.Name)
		self.submitSchedule(sched)
	} else {
		log.Printf("EventJobCreated(%s): no schedule changes made", clusterJob.Name)
	}
}

func (self *Dispatcher) handleEventMachineDeleted(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.Printf("EventMachineDeleted(%s): removing machine from dispatcher's active list", m.BootId)
	delete(self.machines, m.BootId)
}

func (self *Dispatcher) handleEventMachineCreated(event registry.Event) {
	m := event.Payload.(machine.Machine)
	log.Printf("EventMachineCreated(%s): event received", m.BootId)
	self.machines[m.BootId] = m

	sched := NewSchedule()
	for _, clusterJob := range self.jobs {
		name := fmt.Sprintf("%s.%s", m.BootId, clusterJob.Name)
		log.Printf("EventMachineCreated(%s): adding to schedule job=%s machine=%s", m.BootId, name, m.BootId)

		j, _ := job.NewJob(name, nil, clusterJob.Payload)
		sched.Add(*j, m)
	}
	if len(sched) > 0 {
		log.Printf("EventMachineCreated(%s): submitting schedule", m.BootId)
		self.submitSchedule(sched)
	} else {
		log.Printf("EventMachineCreated(%s): no schedule changes made", m.BootId)
	}
	log.Printf("EventMachineCreated(%s): event handler complete", m.BootId)
}
