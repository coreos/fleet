package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
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
	agent   *agent.Agent
	engine  *engine.Engine
	rStream *registry.EventStream
	sStream *systemd.EventStream
	eBus    *event.EventBus
	mach    *machine.CoreOSMachine

	stop chan bool
}

func New(cfg config.Config) (*Server, error) {
	mach, err := newMachineFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	mgr, err := systemd.NewSystemdUnitManager(systemd.DefaultUnitsDirectory)
	if err != nil {
		return nil, err
	}

	eClient, err := etcd.NewClient(cfg.EtcdServers, http.Transport{})
	if err != nil {
		return nil, err
	}

	reg := registry.New(eClient, cfg.EtcdKeyPrefix)

	a, err := newAgentFromConfig(mach, reg, cfg, mgr)
	if err != nil {
		return nil, err
	}

	e := engine.New(reg, mach)

	sStream := systemd.NewEventStream(mgr)

	rStream, err := registry.NewEventStream(eClient, reg)
	if err != nil {
		return nil, err
	}

	aHandler := agent.NewEventHandler(a)
	eHandler := engine.NewEventHandler(e)

	eBus := event.NewEventBus()
	eBus.AddListener("engine", eHandler)
	eBus.AddListener("agent", aHandler)

	return &Server{a, e, rStream, sStream, eBus, mach, nil}, nil
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

func newAgentFromConfig(mach machine.Machine, reg registry.Registry, cfg config.Config, mgr unit.UnitManager) (*agent.Agent, error) {
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

func (s *Server) Run() {
	idx := s.agent.Initialize()

	s.stop = make(chan bool)
	go s.mach.PeriodicRefresh(machineStateRefreshInterval, s.stop)
	go s.rStream.Stream(idx, s.eBus.Dispatch, s.stop)
	go s.sStream.Stream(s.eBus.Dispatch, s.stop)
	go s.agent.Heartbeat(s.stop)

	s.engine.CheckForWork()
}

func (s *Server) Stop() {
	close(s.stop)
}

func (s *Server) Purge() {
	s.agent.Purge()
}

func (s *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: s.agent})
}
