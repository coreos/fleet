package server

import (
	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/config"
	"github.com/coreos/coreinit/engine"
	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type Server struct {
	agent    *agent.Agent
	engine   *engine.Engine
	machine  *machine.Machine
	registry *registry.Registry
	eventBus *event.EventBus
	eventStream *registry.EventStream
}

func New(cfg config.Config) *Server {
	m := machine.New(cfg.BootId, cfg.PublicIP, cfg.Metadata())

	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.WEAK_CONSISTENCY)
	r := registry.New(regClient)

	eb := event.NewEventBus()
	eb.Listen()

	eventClient := etcd.NewClient(cfg.EtcdServers)
	eventClient.SetConsistency(etcd.WEAK_CONSISTENCY)
	es := registry.NewEventStream(eventClient)

	a := agent.New(r, eb, m, cfg.AgentTTL, cfg.UnitPrefix)
	e := engine.New(r, eb, m)

	return &Server{a, e, m, r, eb, es}
}

func (self *Server) Run() {
	go self.agent.Run()
	go self.engine.Run()

	go self.eventBus.Listen()
	go self.eventStream.Stream(self.eventBus.Channel)
}

func (self *Server) Stop() {
	self.agent.Stop()
	self.engine.Stop()

	self.eventStream.Close()
	self.eventBus.Stop()
}

func (self *Server) Purge() {
	self.agent.Purge()
}
