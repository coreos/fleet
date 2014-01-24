package event

import (
	"github.com/coreos/coreinit/machine"
)

type Event struct {
	Type    string
	Payload interface{}
	Context *machine.Machine
}
