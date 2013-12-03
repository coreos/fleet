package engine

import (
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type Engine struct {
	dispatcher *Dispatcher
	watcher    *JobWatcher
	registry   *registry.Registry
	machine    *machine.Machine
}

func New(reg *registry.Registry, events *registry.EventStream, mach *machine.Machine) *Engine {
	scheduler := NewScheduler()
	watcher := NewJobWatcher(reg, scheduler, mach)
	dispatcher := NewDispatcher(reg, events, watcher, mach)
	return &Engine{dispatcher, watcher, reg, mach}
}

func (engine *Engine) Run() {
	engine.watcher.StartHeartbeatThread()
	engine.watcher.StartRefreshThread()

	engine.dispatcher.Listen()
}
