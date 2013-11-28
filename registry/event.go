package registry

import (
	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	EventJobCreated int = iota
	EventJobDeleted
	EventMachineCreated
	EventMachineDeleted
)

type JobEvent struct {
	Type int
	Payload *job.Job
}

type MachineEvent struct {
	Type int
	Payload *machine.Machine
}
