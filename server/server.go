package server

import (
	"encoding/json"
	"errors"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/version"
)

type Server struct {
	agent   *agent.Agent
	engine  *engine.Engine
	eStream *registry.EventStream
	eBus    *event.EventBus

	stop chan bool
}

func New(cfg config.Config) (*Server, error) {
	mach, err := newMachineFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	a, err := newAgentFromConfig(mach, cfg)
	if err != nil {
		return nil, err
	}

	e, err := newEngineFromConfig(mach, cfg)
	if err != nil {
		return nil, err
	}

	eStream, err := newRegistryEventStreamFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	eHandler := engine.NewEventHandler(e)

	eBus := event.NewEventBus()
	eBus.AddListener("engine", eHandler)

	return &Server{a, e, eStream, eBus, nil}, nil
}

func newRegistryEventStreamFromConfig(cfg config.Config) (*registry.EventStream, error) {
	eClient := etcd.NewClient(cfg.EtcdServers)
	eClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	reg := registry.New(eClient, cfg.EtcdKeyPrefix)
	return registry.NewEventStream(eClient, reg)
}

func newMachineFromConfig(cfg config.Config) (*machine.Machine, error) {
	state := machine.MachineState{
		PublicIP: cfg.PublicIP,
		Metadata: cfg.Metadata(),
		Version:  version.Version,
	}

	mach := machine.New(state)
	mach.RefreshState()

	if mach.State().ID == "" {
		return nil, errors.New("unable to determine local machine ID")
	}

	return mach, nil
}

func newAgentFromConfig(mach *machine.Machine, cfg config.Config) (*agent.Agent, error) {
	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	reg := registry.New(regClient, cfg.EtcdKeyPrefix)

	eClient := etcd.NewClient(cfg.EtcdServers)
	eClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	eStream, err := registry.NewEventStream(eClient, reg)
	if err != nil {
		return nil, err
	}

	var verifier *sign.SignatureVerifier
	if cfg.VerifyUnits {
		var err error
		verifier, err = sign.NewSignatureVerifierFromAuthorizedKeysFile(cfg.AuthorizedKeysFile)
		if err != nil {
			log.Errorln("Failed to get any key from authorized key file in verify_units mode:", err)
			verifier = sign.NewSignatureVerifier()
		}
	}

	return agent.New(reg, eStream, mach, cfg.AgentTTL, verifier)
}

func newEngineFromConfig(mach *machine.Machine, cfg config.Config) (*engine.Engine, error) {
	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	reg := registry.New(regClient, cfg.EtcdKeyPrefix)
	return engine.New(reg, mach), nil
}

func (s *Server) Run() {
	s.agent.Run()

	s.stop = make(chan bool)

	go s.eBus.Listen(s.stop)
	go s.eStream.Stream(0, s.eBus.Channel, s.stop)

	s.engine.CheckForWork()
}

func (s *Server) Stop() {
	close(s.stop)
	s.agent.Stop()
}

func (s *Server) Purge() {
	s.agent.Purge()
}

func (s *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: s.agent})
}
