package heart

import (
	"time"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

type Heart interface {
	Beat(time.Duration) (uint64, error)
	Clear() error
}

func New(reg registry.Registry, mach machine.Machine) Heart {
	return &machineHeart{reg, mach}
}

type machineHeart struct {
	reg  registry.Registry
	mach machine.Machine
}

func (h *machineHeart) Beat(ttl time.Duration) (uint64, error) {
	return h.reg.SetMachineState(h.mach.State(), ttl)
}

func (h *machineHeart) Clear() error {
	return h.reg.RemoveMachineState(h.mach.State().ID)
}
