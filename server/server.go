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
	etcdClient := etcd.NewClient(cfg.EtcdServers)
	etcdClient.SetConsistency(etcd.WEAK_CONSISTENCY)
	r := registry.New(etcdClient)
	es := registry.NewEventStream(etcdClient)

	m := machine.New(cfg.BootId)

	a := agent.New(r, es, m, "")
	e := engine.New(r, es, m)

	return &Server{a, e, m, r, es}
}

func (self *Server) Run() {
	go self.agent.Run()
	go self.engine.Run()
}

func (self *Server) Configure(cfg *config.Config) {
	if cfg.BootId != self.machine.BootId {
		self.machine = machine.New(cfg.BootId)
		self.agent.Stop()
		self.agent = agent.New(self.registry, self.events, self.machine, "")
		go self.agent.Run()
	}
}
