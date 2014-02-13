package event

import (
	"github.com/coreos/fleet/machine"
)

type EventListener struct {
	Context *machine.Machine
	Handler interface{}
}

func (self *EventListener) String() string {
	if self.Context != nil {
		return self.Context.State().BootId
	} else {
		return "N/A"
	}
}
