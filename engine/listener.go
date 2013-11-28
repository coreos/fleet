package engine

import (
	"fmt"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type EventListener struct {
	registry *registry.Registry
	jobs     []job.Job
	machines []machine.Machine
}

func NewEventListener(reg *registry.Registry) *EventListener {
	return &EventListener{registry: reg}
}

func (el *EventListener) Listen() {
	go el.listenForJobEvents()
	el.listenForMachineEvents()
}

func (el *EventListener) listenForJobEvents() {
	eventchan := make(chan registry.JobEvent)
	el.registry.RegisterJobListener(eventchan)

	for true {
		event := <-eventchan
		clusterJob := event.Payload
		if event.Type == registry.EventJobCreated {
			el.jobs = append(el.jobs, *clusterJob)
			for _, m := range el.registry.GetActiveMachines() {
				name := fmt.Sprintf("%s.%s", m.BootId, clusterJob.Name)
				job, _ := job.NewJob(name, nil, clusterJob.Payload)
				el.registry.ScheduleMachineJob(job, &m)
			}
		}
	}
}

func (el *EventListener) listenForMachineEvents() {
	eventchan := make(chan registry.MachineEvent)
	el.registry.RegisterMachineListener(eventchan)

	for true {
		event := <-eventchan
		m := event.Payload
		if event.Type == registry.EventMachineCreated {
			el.machines = append(el.machines, *m)
			for _, clusterJob := range el.jobs {
				name := fmt.Sprintf("%s.%s", m.BootId, clusterJob.Name)
				j, _ := job.NewJob(name, nil, clusterJob.Payload)
				el.registry.ScheduleMachineJob(j, m)
			}
		}
	}
}
