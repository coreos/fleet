package engine

import (
	"fmt"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type EventConsumer struct {
	registry *registry.Registry
	jobs     []job.Job
	machines []machine.Machine
}

func NewEventConsumer(reg *registry.Registry) *EventConsumer {
	return &EventConsumer{registry: reg}
}

func (ec *EventConsumer) Listen() {
	go ec.listenForJobEvents()
	ec.listenForMachineEvents()
}

func (ec *EventConsumer) listenForJobEvents() {
	eventchan := make(chan registry.JobEvent)
	ec.registry.RegisterJobListener(eventchan)

	for true {
		event := <-eventchan
		clusterJob := event.Payload
		if event.Type == registry.EventJobCreated {
			ec.jobs = append(ec.jobs, *clusterJob)
			for _, m := range ec.registry.GetActiveMachines() {
				name := fmt.Sprintf("%s.%s", m.BootId, clusterJob.Name)
				job, _ := job.NewJob(name, nil, clusterJob.Payload)
				ec.registry.ScheduleMachineJob(job, &m)
			}
		}
	}
}

func (ec *EventConsumer) listenForMachineEvents() {
	eventchan := make(chan registry.MachineEvent)
	ec.registry.RegisterMachineListener(eventchan)

	for true {
		event := <-eventchan
		m := event.Payload
		if event.Type == registry.EventMachineCreated {
			for _, clusterJob := range ec.jobs {
				name := fmt.Sprintf("%s.%s", m.BootId, clusterJob.Name)
				j, _ := job.NewJob(name, nil, clusterJob.Payload)
				ec.registry.ScheduleMachineJob(j, m)
			}
		}
	}
}
