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
	"github.com/coreos/fleet/heart"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
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
	agent       *agent.Agent
	aReconciler *agent.AgentReconciler
	usPub       *agent.UnitStatePublisher
	usGen       *unit.UnitStateGenerator
	engine      *engine.Engine
	mach        *machine.CoreOSMachine
	hrt         heart.Heart
	mon         *heart.Monitor
	api         *api.Server

	engineReconcileInterval time.Duration

	stop chan bool
}

func New(cfg config.Config) (*Server, error) {
	if cfg.VerifyUnits {
		log.Error("Config option verify_units is deprecated - ignoring")
	}
	if len(cfg.AuthorizedKeysFile) > 0 {
		log.Error("Config option authorized_keys_file is deprecated - ignoring")
	}
	mgr, err := systemd.NewSystemdUnitManager(systemd.DefaultUnitsDirectory)
	if err != nil {
		return nil, err
	}

	mach, err := newMachineFromConfig(cfg, mgr)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := etcd.ReadTLSConfigFiles(cfg.EtcdCAFile, cfg.EtcdCertFile, cfg.EtcdKeyFile)
	if err != nil {
		return nil, err
	}

	eTrans := http.Transport{TLSClientConfig: tlsConfig}
	timeout := time.Duration(cfg.EtcdRequestTimeout*1000) * time.Millisecond
	eClient, err := etcd.NewClient(cfg.EtcdServers, eTrans, timeout)
	if err != nil {
		return nil, err
	}

	reg := registry.New(eClient, cfg.EtcdKeyPrefix)

	pub := agent.NewUnitStatePublisher(mgr, reg, mach)
	gen := unit.NewUnitStateGenerator(mgr)

	a, err := newAgentFromConfig(mach, reg, cfg, mgr, gen)
	if err != nil {
		return nil, err
	}

	ar, err := newAgentReconcilerFromConfig(reg, eClient, cfg)
	if err != nil {
		return nil, err
	}

	e, err := newEngineFromConfig(reg, eClient, mach, cfg)
	if err != nil {
		return nil, err
	}

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

	eIval := time.Duration(cfg.EngineReconcileInterval*1000) * time.Millisecond

	srv := Server{
		agent:       a,
		aReconciler: ar,
		usGen:       gen,
		usPub:       pub,
		engine:      e,
		mach:        mach,
		hrt:         hrt,
		mon:         mon,
		api:         apiServer,
		stop:        nil,
		engineReconcileInterval: eIval,
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

func newMachineFromConfig(cfg config.Config, mgr unit.UnitManager) (*machine.CoreOSMachine, error) {
	state := machine.MachineState{
		PublicIP: cfg.PublicIP,
		Metadata: cfg.Metadata(),
		Version:  version.Version,
	}

	mach := machine.NewCoreOSMachine(state, mgr)
	mach.Refresh()

	if mach.State().ID == "" {
		return nil, errors.New("unable to determine local machine ID")
	}

	return mach, nil
}

func newEngineFromConfig(reg registry.Registry, eClient etcd.Client, mach machine.Machine, cfg config.Config) (*engine.Engine, error) {
	listen := []registry.Event{registry.JobTargetChangeEvent, registry.JobTargetStateChangeEvent}
	rStream, err := registry.NewEtcdEventStream(eClient, cfg.EtcdKeyPrefix, listen)
	if err != nil {
		return nil, err
	}

	e := engine.New(reg, rStream, mach)
	return e, nil
}

func newAgentFromConfig(mach machine.Machine, reg registry.Registry, cfg config.Config, mgr unit.UnitManager, uGen *unit.UnitStateGenerator) (*agent.Agent, error) {
	return agent.New(mgr, uGen, reg, mach, cfg.AgentTTL)
}

func newAgentReconcilerFromConfig(reg registry.Registry, eClient etcd.Client, cfg config.Config) (*agent.AgentReconciler, error) {
	listen := []registry.Event{registry.JobTargetChangeEvent}
	rStream, err := registry.NewEtcdEventStream(eClient, cfg.EtcdKeyPrefix, listen)
	if err != nil {
		return nil, err
	}

	return agent.NewReconciler(reg, rStream)
}

func (s *Server) Run() {
	log.Infof("Establishing etcd connectivity")

	var err error
	for sleep := time.Second; ; sleep = pkg.ExpBackoff(sleep, time.Minute) {
		_, err = s.hrt.Beat(s.mon.TTL)
		if err == nil {
			break
		}
		time.Sleep(sleep)
	}

	log.Infof("Starting server components")

	s.stop = make(chan bool)

	go s.Monitor()
	go s.api.Available(s.stop)
	go s.mach.PeriodicRefresh(machineStateRefreshInterval, s.stop)
	go s.agent.Heartbeat(s.stop)
	go s.aReconciler.Run(s.agent, s.stop)
	go s.engine.Run(s.engineReconcileInterval, s.stop)

	beatchan := make(chan *unit.UnitStateHeartbeat)
	go s.usGen.Run(beatchan, s.stop)
	go s.usPub.Run(beatchan, s.stop)
}

// Monitor tracks the health of the Server. If the Server is ever deemed
// unhealthy, the Server is restarted.
func (s *Server) Monitor() {
	err := s.mon.Monitor(s.hrt, s.stop)
	if err != nil {
		log.Errorf("Server monitor triggered: %v", err)

		s.Stop()
		s.Run()
	}
}

func (s *Server) Stop() {
	close(s.stop)
}

func (s *Server) Purge() {
	s.aReconciler.Purge(s.agent)
	s.engine.Purge()
	s.hrt.Clear()
}

func (s *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: s.agent})
}
