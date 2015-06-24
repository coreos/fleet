// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	etcd "github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/etcd/client"
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/activation"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/api"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/heart"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/pkg/lease"
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
	agent         *agent.Agent
	aReconciler   *agent.AgentReconciler
	usPub         *agent.UnitStatePublisher
	usGen         *unit.UnitStateGenerator
	engine        *engine.Engine
	mach          *machine.CoreOSMachine
	hrt           heart.Heart
	mon           *heart.Monitor
	api           *api.Server
	disableEngine bool

	engineReconcileInterval time.Duration

	stop chan bool
}

func New(cfg config.Config) (*Server, error) {
	agentTTL, err := time.ParseDuration(cfg.AgentTTL)
	if err != nil {
		return nil, err
	}

	mgr, err := systemd.NewSystemdUnitManager(systemd.DefaultUnitsDirectory)
	if err != nil {
		return nil, err
	}

	mach, err := newMachineFromConfig(cfg, mgr)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := pkg.ReadTLSConfigFiles(cfg.EtcdCAFile, cfg.EtcdCertFile, cfg.EtcdKeyFile)
	if err != nil {
		return nil, err
	}

	eCfg := etcd.Config{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		Endpoints: cfg.EtcdServers,
	}
	eClient, err := etcd.New(eCfg)
	if err != nil {
		return nil, err
	}

	etcdRequestTimeout := time.Duration(cfg.EtcdRequestTimeout*1000) * time.Millisecond
	kAPI := etcd.NewKeysAPI(eClient)
	reg := registry.NewEtcdRegistry(kAPI, cfg.EtcdKeyPrefix, etcdRequestTimeout)

	pub := agent.NewUnitStatePublisher(reg, mach, agentTTL)
	gen := unit.NewUnitStateGenerator(mgr)

	a := agent.New(mgr, gen, reg, mach, agentTTL)

	rStream := registry.NewEtcdEventStream(kAPI, cfg.EtcdKeyPrefix)
	lManager := lease.NewEtcdLeaseManager(kAPI, cfg.EtcdKeyPrefix, etcdRequestTimeout)

	ar := agent.NewReconciler(reg, rStream)

	e := engine.New(reg, lManager, rStream, mach)

	listeners, err := activation.Listeners(false)
	if err != nil {
		return nil, err
	}

	hrt := heart.New(reg, mach)
	mon := heart.NewMonitor(agentTTL)

	apiServer := api.NewServer(listeners, api.NewServeMux(reg, cfg.TokenLimit))
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
		disableEngine:           cfg.DisableEngine,
	}

	return &srv, nil
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
	if s.disableEngine {
		log.Info("Not starting engine; disable-engine is set")
	} else {
		go s.engine.Run(s.engineReconcileInterval, s.stop)
	}

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
	s.usPub.Purge()
	s.engine.Purge()
	s.hrt.Clear()
}

func (s *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Agent              *agent.Agent
		UnitStatePublisher *agent.UnitStatePublisher
		UnitStateGenerator *unit.UnitStateGenerator
	}{
		Agent:              s.agent,
		UnitStatePublisher: s.usPub,
		UnitStateGenerator: s.usGen,
	})
}
