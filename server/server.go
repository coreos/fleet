package server

import (
	"encoding/json"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/config"
	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
)

type Server struct {
	agent  *agent.Agent
	engine *engine.Engine
}

func New(cfg config.Config) (*Server, error) {
	a, err := newAgentFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	e := newEngineFromConfig(cfg)

	return &Server{a, e}, nil
}

func newAgentFromConfig(cfg config.Config) (*agent.Agent, error) {
	mach := machine.New(cfg.BootID, cfg.PublicIP, cfg.Metadata())

	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	reg := registry.New(regClient)

	eClient := etcd.NewClient(cfg.EtcdServers)
	eClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	eStream := registry.NewEventStream(eClient, reg)

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

func newEngineFromConfig(cfg config.Config) *engine.Engine {
	mach := machine.New(cfg.BootID, cfg.PublicIP, cfg.Metadata())

	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	reg := registry.New(regClient)

	eClient := etcd.NewClient(cfg.EtcdServers)
	eClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	eStream := registry.NewEventStream(eClient, reg)

	return engine.New(reg, eStream, mach)
}

func (self *Server) Run() {
	self.agent.Run()
	self.engine.Run()
}

func (self *Server) Stop() {
	self.agent.Stop()
	self.engine.Stop()
}

func (self *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: self.agent})
}
