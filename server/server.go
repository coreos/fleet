package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/activation"
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/api"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/heart"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
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
	hrt     heart.Heart
	mon     *heart.Monitor
	api     *api.Server

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

	eBus := event.NewEventBus()
	aHandler := agent.NewEventHandler(a)
	eBus.AddListener("agent", aHandler)

	listeners, err := activation.Listeners(false)
	if err != nil {
		return nil, err
	}

	hrt, mon, err := newHeartMonitorFromConfig(mach, reg, cfg)
	if err != nil {
		return nil, err
	}

	apiServer := api.NewServer(listeners, api.NewServeMux(reg))
	apiServer.Serve()

	srv := Server{
		agent:   a,
		engine:  e,
		rStream: rStream,
		sStream: sStream,
		eBus:    eBus,
		mach:    mach,
		hrt:     hrt,
		mon:     mon,
		api:     apiServer,
		stop:    nil,
	}

	return &srv, nil
}

func newHeartMonitorFromConfig(mach machine.Machine, reg registry.Registry, cfg config.Config) (hrt heart.Heart, mon *heart.Monitor, err error) {
	var ttl time.Duration
	ttl, err = time.ParseDuration(cfg.AgentTTL)
	if err != nil {
		return
	}

	hrt = heart.New(reg, mach)
	mon = heart.NewMonitor(ttl)
	return
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
	log.Infof("Establishing etcd connectivity")

	var idx uint64
	var err error
	for sleep := time.Second; ; sleep = pkg.ExpBackoff(sleep, time.Minute) {
		idx, err = s.hrt.Beat(s.mon.TTL)
		if err == nil {
			break
		}
		time.Sleep(sleep)
	}

	log.Infof("Starting server components")

	//TODO(bcwaldon): initialize Agent at the index yielded by the first heartbeat
	s.agent.Initialize()

	s.stop = make(chan bool)

	go s.Monitor()
	go s.api.Available(s.stop)
	go s.mach.PeriodicRefresh(machineStateRefreshInterval, s.stop)
	go s.rStream.Stream(idx, s.eBus.Dispatch, s.stop)
	go s.sStream.Stream(s.eBus.Dispatch, s.stop)
	go s.agent.Heartbeat(s.stop)
	go s.engine.Run(s.stop)
}

// Monitor tracks the health of the Server. If the Server is ever deemed
// unhealthy, the Server is stopped, purged, and started up again.
func (s *Server) Monitor() {
	err := s.mon.Monitor(s.hrt, s.stop)
	if err != nil {
		log.Errorf("Server monitor triggered: %v", err)
		s.Stop()
		s.Purge()
		s.Run()
	}
}

func (s *Server) Stop() {
	log.Infof("Stopping server components")
	close(s.stop)
}

func (s *Server) Purge() {
	s.agent.Purge()
}

func (s *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: s.agent})
}
