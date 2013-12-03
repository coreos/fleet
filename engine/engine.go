package engine

import (
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type Engine struct {
	dispatcher *Dispatcher
	machine    *machine.Machine
	registry   *registry.Registry
}

func New(reg *registry.Registry, events *registry.EventStream, mach *machine.Machine) *Engine {
	scheduler := NewScheduler()
	dispatcher := NewDispatcher(reg, events, scheduler, mach)
	engine := Engine{dispatcher, mach, reg}
	return &engine
}

func (engine *Engine) Run() {
	engine.dispatcher.Listen()
}
