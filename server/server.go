package server

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/heartbeat"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
	"github.com/coreos/fleet/version"
)

const (
	// machineStateRefreshInterval is the amount of time the server will
	// wait before each attempt to refresh the local machine state
	machineStateRefreshInterval = time.Minute
)

type Server struct {
	reg     registry.Registry
	agent   *agent.Agent
	engine  *engine.Engine
	rStream *registry.EventStream
	sStream *systemd.EventStream
	eBus    *event.EventBus
	mach    *machine.CoreOSMachine
	heart   *heartbeat.Heart

	stop chan bool
}

func New(cfg config.Config) (*Server, error) {
	mach, err := newMachineFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	heart, err := newHeartFromConfig(cfg, mach)
	if err != nil {
		return nil, err
	}

	reg := newRegistryFromConfig(cfg)

	mgr, err := systemd.NewSystemdUnitManager(systemd.DefaultUnitsDirectory)
	if err != nil {
		return nil, err
	}

	a, err := newAgentFromConfig(mach, cfg, mgr)
	if err != nil {
		return nil, err
	}

	e, err := newEngineFromConfig(mach, cfg)
	if err != nil {
		return nil, err
	}

	sStream := systemd.NewEventStream(mgr)

	rStream, err := newRegistryEventStreamFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	aHandler := agent.NewEventHandler(a)
	eHandler := engine.NewEventHandler(e)

	eBus := event.NewEventBus()
	eBus.AddListener("engine", eHandler)
	eBus.AddListener("agent", aHandler)

	return &Server{reg, a, e, rStream, sStream, eBus, mach, heart, nil}, nil
}

func newEtcdClientFromConfig(cfg config.Config) *etcd.Client {
	c := etcd.NewClient(cfg.EtcdServers)
	c.SetConsistency(etcd.STRONG_CONSISTENCY)
	return c
}

func newRegistryFromConfig(cfg config.Config) registry.Registry {
	eClient := newEtcdClientFromConfig(cfg)
	return registry.New(eClient, cfg.EtcdKeyPrefix)
}

func newRegistryEventStreamFromConfig(cfg config.Config) (*registry.EventStream, error) {
	eClient := newEtcdClientFromConfig(cfg)
	reg := registry.New(eClient, cfg.EtcdKeyPrefix)
	return registry.NewEventStream(eClient, reg)
}

func newMachineFromConfig(cfg config.Config) (*machine.CoreOSMachine, error) {
	state := machine.MachineState{
		PublicIP: cfg.PublicIP,
		Metadata: cfg.Metadata(),
		Version:  version.Version,
	}

	mach := machine.NewCoreOSMachine(state)
	mach.Refresh()

	if mach.State().ID == "" {
		return nil, errors.New("unable to determine local machine ID")
	}

	return mach, nil
}

func newHeartFromConfig(cfg config.Config, mach machine.Machine) (*heartbeat.Heart, error) {
	ttl, err := time.ParseDuration(cfg.AgentTTL)
	if err != nil {
		return nil, err
	}

	ival := ttl / 2

	eClient := newEtcdClientFromConfig(cfg)
	reg := registry.New(eClient, cfg.EtcdKeyPrefix)

	fn := heartbeat.MachineHeartbeatFunc(reg, mach, ttl)
	return heartbeat.NewHeart(fn, ival), nil
}

func newAgentFromConfig(mach machine.Machine, cfg config.Config, mgr unit.UnitManager) (*agent.Agent, error) {
	regClient := newEtcdClientFromConfig(cfg)
	reg := registry.New(regClient, cfg.EtcdKeyPrefix)

	var verifier *sign.SignatureVerifier
	if cfg.VerifyUnits {
		var err error
		verifier, err = sign.NewSignatureVerifierFromAuthorizedKeysFile(cfg.AuthorizedKeysFile)
		if err != nil {
			log.Errorln("Failed to get any key from authorized key file in verify_units mode:", err)
			verifier = sign.NewSignatureVerifier()
		}
	}

	return agent.New(mgr, reg, mach, cfg.AgentTTL, verifier)
}

func newEngineFromConfig(mach machine.Machine, cfg config.Config) (*engine.Engine, error) {
	regClient := newEtcdClientFromConfig(cfg)
	reg := registry.New(regClient, cfg.EtcdKeyPrefix)
	return engine.New(reg, mach), nil
}

func (s *Server) Run() {
	idx := s.agent.Initialize()

	asyncDispatch := func(ev *event.Event) {
		go s.eBus.Dispatch(ev)
	}

	s.stop = make(chan bool)

	go s.mach.PeriodicRefresh(machineStateRefreshInterval, s.stop)
	go s.heart.Start(s.stop)

	go s.rStream.Stream(idx, asyncDispatch, s.stop)
	go s.sStream.Stream(asyncDispatch, s.stop)
	go s.agent.Heartbeat(s.stop)

	s.engine.CheckForWork()
}

func (s *Server) Stop() {
	close(s.stop)
}

func (s *Server) Purge() {
	// Continue heartbeating the machine state so the rest of the
	// cluster will allow the Agent to purge its own state.
	stop := make(chan bool)
	go s.heart.Start(stop)

	s.agent.Purge()

	close(stop)

	machID := s.mach.State().ID
	log.Info("Removing Machine(%s) from Registry", machID)
	err := s.reg.RemoveMachineState(machID)
	if err != nil {
		log.Errorf("Failed to remove Machine(%s) from Registry: %v", machID, err)
	}
}

func (s *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: s.agent})
}
