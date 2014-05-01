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
	mach := machine.New(cfg.BootID, cfg.PublicIP, cfg.Metadata())

	regClient := etcd.NewClient(cfg.EtcdServers)
	regClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	reg := registry.New(regClient)

	aEventClient := etcd.NewClient(cfg.EtcdServers)
	aEventClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	aEventStream := registry.NewEventStream(aEventClient, reg)

	var verifier *sign.SignatureVerifier
	if cfg.VerifyUnits {
		var err error
		verifier, err = sign.NewSignatureVerifierFromAuthorizedKeysFile(cfg.AuthorizedKeysFile)
		if err != nil {
			log.Errorln("Failed to get any key from authorized key file in verify_units mode:", err)
			verifier = sign.NewSignatureVerifier()
		}
	}

	a, err := agent.New(reg, aEventStream, mach, cfg.AgentTTL, verifier)
	if err != nil {
		log.Errorf("Error creating Agent")
		return nil, err
	}

	eEventClient := etcd.NewClient(cfg.EtcdServers)
	eEventClient.SetConsistency(etcd.STRONG_CONSISTENCY)
	eEventStream := registry.NewEventStream(eEventClient, reg)

	e := engine.New(reg, eEventStream, mach)

	return &Server{a, e}, nil
}

func (self *Server) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ Agent *agent.Agent }{Agent: self.agent})
}

func (self *Server) Run() {
	self.agent.Run()
	self.engine.Run()
}

func (self *Server) Stop() {
	self.agent.Stop()
	self.engine.Stop()
}

func (self *Server) Purge() {
	self.agent.Purge()
}
