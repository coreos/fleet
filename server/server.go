package server

import (
	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/agent"
	"github.com/coreos/coreinit/config"
	"github.com/coreos/coreinit/engine"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type Server struct {
	agent    *agent.Agent
	engine   *engine.Engine
	machine  *machine.Machine
	registry *registry.Registry
	events   *registry.EventStream
}

func New(cfg config.Config) *Server {
	m := machine.New(cfg.BootId, cfg.PublicIP, cfg.Metadata())

	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.WEAK_CONSISTENCY)
	r := registry.New(regClient)

	eventClient := etcd.NewClient(cfg.EtcdServers)
	eventClient.SetConsistency(etcd.WEAK_CONSISTENCY)
	es := registry.NewEventStream(eventClient, r)
	es.Open()

	a := agent.New(r, es, m, "", cfg.UnitPrefix)
	e := engine.New(r, es, m)

	return &Server{a, e, m, r, es}
}

func (self *Server) Run() {
	go self.agent.Run()
	go self.engine.Run()
}

func (self *Server) Configure(cfg *config.Config) {
	if cfg.BootId != self.machine.BootId {
		self.machine = machine.New(cfg.BootId, cfg.PublicIP, cfg.Metadata())
		self.agent.Stop()
		self.agent = agent.New(self.registry, self.events, self.machine, "", cfg.UnitPrefix)
		go self.agent.Run()
	}
}
