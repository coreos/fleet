package engine

import (
	"time"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

const (
	DefaultRequestClaimTTL = "4s"
)

type Engine struct {
	registry *registry.Registry
	events   *event.EventBus
	machine  *machine.Machine
	claimTTL time.Duration

	stop     chan bool
}

func New(reg *registry.Registry, events *event.EventBus, mach *machine.Machine) *Engine {
	claimTTL, _ := time.ParseDuration(DefaultRequestClaimTTL)
	return &Engine{reg, events, mach, claimTTL, nil}
}

func (self *Engine) Run() {
	self.stop = make(chan bool)

	handler := NewEventHandler(self)
	self.events.AddListener("engine", self.machine, handler)

	// Block until we receive a stop signal
	<-self.stop

	self.events.RemoveListener("engine", self.machine)
}

func (self *Engine) Stop() {
	log.V(1).Info("Stopping Engine")
	close(self.stop)
}

func (self *Engine) Registry() *registry.Registry {
	return self.registry
}

func (self *Engine) Machine() *machine.Machine {
	return self.machine
}

func (self *Engine) ClaimTTL() time.Duration {
	return self.claimTTL
}
