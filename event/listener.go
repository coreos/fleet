package event

import (
	"github.com/coreos/coreinit/machine"
)

type EventListener struct {
	Context *machine.Machine
	Handler interface{}
}

func (self *EventListener) String() string {
	if self.Context != nil {
		return self.Context.BootId
	} else {
		return "N/A"
	}
}
